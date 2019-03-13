package grpcweb

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
)

const (
	// header names
	hAccept        = "accept"
	hContentLength = "content-length"
	hContentType   = "content-type"
	hTE            = "te"
	hTrailer       = "trailer"
	// content-type s
	ctGRPC        = "application/grpc"
	ctGRPCWeb     = "application/grpc-web"
	ctGRPCWebText = "application/grpc-web-text"
	// te: trailers
	teTrailers = "trailers"
)

// ========================== server / wrapper

type grpcWeb struct {
	next http.Handler
}

// New returns a new http.Handler wrapping the provided handler
// transforms grpc-web <-> grpc
func New(next http.Handler) http.Handler {
	return &grpcWeb{next}
}

// ServeHTTP implements http.Handler
func (g *grpcWeb) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var writeTrailers func()
	if strings.HasPrefix(req.Header.Get(hContentType), ctGRPCWeb) {
		modifyRequest(req)
		rw, writeTrailers = newResponseWriter(rw, req.Header.Get(hAccept))
		defer writeTrailers()
	}
	g.next.ServeHTTP(rw, req)
}

// ======================== request modifier

// modifyRequest transforms a grpc-web request to a grpc request
// TODO: check if still works when client side streaming is available
func modifyRequest(req *http.Request) {
	// force -> http2
	req.ProtoMajor = 2
	req.ProtoMinor = 0

	ct := req.Header.Get(hContentType)
	ctp := ctGRPCWeb

	if strings.HasPrefix(ct, ctGRPCWebText) {
		req.Body = readCloser{base64.NewDecoder(base64.StdEncoding, req.Body), req.Body}
		ctp = ctGRPCWebText
	}

	req.Header.Del(hContentLength)
	req.Header.Set(hContentType, strings.Replace(ct, ctp, ctGRPC, 1))
	req.Header.Set(hTE, teTrailers)
	return
}

// readCloser implements io.ReadCloser with io.Reader and io.Closer
type readCloser struct {
	io.Reader
	io.Closer
}

//=========================== response modifier

// responseWriter implements http.ResponseWriter, http.CloseNotifier, http.Flusher
// transforms a grpc reponse to a grpc-web response
// call writeTrailers after completion
type responseWriter struct {
	http.ResponseWriter
	header        http.Header
	headerWritten bool
	text          bool

	// base64 encoder buffer
	buf []byte
}

// newResponseWriter creates a new responseWriter
func newResponseWriter(rw http.ResponseWriter, accept string) (r *responseWriter, writeTrailers func()) {
	r = &responseWriter{
		ResponseWriter: rw,
		header:         make(http.Header),
		text:           strings.HasPrefix(accept, ctGRPCWebText),
	}
	return r, r.writeTrailers
}

// prepareHeaders syncs (internal) headers into the ResponseWriter
func (r *responseWriter) prepareHeaders() {
	r.headerWritten = true
	for k, v := range r.header {
		k := strings.ToLower(k)
		if k == hContentType {
			ct := ctGRPCWeb
			if r.text {
				ct = ctGRPCWebText
			}
			r.ResponseWriter.Header().Set(k, strings.Replace(r.header.Get(k), ctGRPC, ct, 1))
			continue
		}
		if strings.HasPrefix(k, strings.ToLower(http.TrailerPrefix)) {
			k = strings.TrimPrefix(k, http.TrailerPrefix)
		}
		for _, vv := range v {
			r.ResponseWriter.Header().Add(k, vv)
		}
	}
}

// WriteHeader implements http.ResponseWriter
// ensures headers are prepared for a write
func (r *responseWriter) WriteHeader(statusCode int) {
	r.prepareHeaders()
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write implements http.ResponseWriter
// encodes in base64 if necessary
// has internal buffer
func (r *responseWriter) Write(p []byte) (int, error) {
	if !r.headerWritten {
		r.prepareHeaders()
	}

	if r.text {
		r.buf = append(r.buf, p...)
		r.ResponseWriter.Write(r.encodeBase64())
		return len(p), nil
	}
	return r.ResponseWriter.Write(p)
}

// encodeBase64 reads the internal write buffer,
// decodes into grpc frames (unrelated to http2 frames)
// returns base64 encoded representation, separated by \r\n
// TODO: figure out if there are any unsupported flags
func (r *responseWriter) encodeBase64() []byte {
	buf := &bytes.Buffer{}
	for len(r.buf) >= 5 { // min frame size
		frameLen := int(binary.BigEndian.Uint32(r.buf[1:5])) + 5
		if len(r.buf) < frameLen {
			break // not enough data, wait for buffer
		}
		enc := base64.NewEncoder(base64.StdEncoding, buf)
		enc.Write(r.buf[:frameLen])
		enc.Close() // flush
		r.buf = r.buf[frameLen:]
	}
	return buf.Bytes()
}

// writeTrailers writes any trailers at the end of a response
// or in the headers if there is no body
// TODO: close http2 conns (or is this the reponsibility of the wrapped handler?)
func (r *responseWriter) writeTrailers() {
	if r.headerWritten {
		// extract unwritten trailers from header
		skip := map[string]bool{hTrailer: true}
		for k := range r.ResponseWriter.Header() {
			skip[strings.ToLower(k)] = true
		}
		buf := bytes.NewBuffer([]byte{1 << 7, 0, 0, 0, 0}) // traler frame header
		r.header.WriteSubset(buf, skip)

		r.buf = buf.Bytes()
		binary.BigEndian.PutUint32(r.buf[1:5], uint32(len(r.buf)-5))
		r.ResponseWriter.Write(r.encodeBase64())
	} else {
		// trailers included in header
		r.WriteHeader(http.StatusOK)
	}
	r.Flush()
}

// Flush implements http.Flusher
func (r *responseWriter) Flush() {
	r.ResponseWriter.(http.Flusher).Flush()
}

// CloseNotify implements http.CloseNotifier
func (r *responseWriter) CloseNotify() <-chan bool {
	return r.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// Header implements http.ResponseWriter
func (r *responseWriter) Header() http.Header {
	return r.header
}
