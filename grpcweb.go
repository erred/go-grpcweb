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
	// content types
	ctHeader      = "content-type"
	ctGRPC        = "application/grpc"
	ctGRPCWeb     = "application/grpc-web"
	ctGRPCWebText = "application/grpc-web-text"

	trailerFrame = 1 << 7 // uncompressed
)

// ========================= utility

func isGRPCWebRequest(req *http.Request) bool {
	return strings.HasPrefix(req.Header.Get("content-type"), "application/grpc-web")
}

func isGRPCWebTextRequest(req *http.Request) bool {
	return strings.HasPrefix(req.Header.Get(ctHeader), ctGRPCWebText)
}

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
	var finisher func()
	if isGRPCWebRequest(req) {
		modifyRequest(req)
		rw, finisher = newResponseWriter(rw, req.Header.Get("accept"))
		// _ = finisher
		defer finisher()
	}
	g.next.ServeHTTP(rw, req)
}

// ======================== request modifier

// modifyRequest transforms a grpc-web request to a grpc request
func modifyRequest(req *http.Request) {
	// force -> http2
	req.ProtoMajor = 2
	req.ProtoMinor = 0

	if isGRPCWebTextRequest(req) {
		// TODO: check if still works when client side streaming is available
		req.Body = readCloser{base64.NewDecoder(base64.StdEncoding, req.Body), req.Body}
	}

	req.Header.Del("content-length")
	req.Header.Set("content-type", ctGRPC)
	req.Header.Set("te", "trailers")
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
	bodyWritten   bool
	text          bool

	// base64 encoder buffer
	buf []byte
}

// newResponseWriter creates a new responseWriter
// response encoding is decided by accept (request header)
func newResponseWriter(rw http.ResponseWriter, accept string) (r *responseWriter, writeTrailers func()) {
	r = &responseWriter{
		ResponseWriter: rw,
		header:         make(http.Header),
	}
	if strings.HasPrefix(accept, ctGRPCWebText) {
		r.text = true
	}
	return r, r.writeTrailers
}

// prepareHeaders syncs (internal) headers into the ResponseWriter
func (r *responseWriter) prepareHeaders() {
	for k, v := range r.header {
		k := strings.ToLower(k)
		if k == ctHeader {
			if r.text {
				v = []string{ctGRPCWebText}
			} else {
				v = []string{ctGRPCWeb}
			}
		}
		if strings.HasPrefix(k, strings.ToLower(http.TrailerPrefix)) {
			k = strings.TrimPrefix(k, http.TrailerPrefix)
		}
		for _, vv := range v {
			r.ResponseWriter.Header().Add(k, vv)
		}
	}
	r.headerWritten = true
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
	r.bodyWritten = true
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
func (r *responseWriter) encodeBase64() []byte {
	var l uint32
	buf := &bytes.Buffer{}
	for len(r.buf) > 5 { // min frame size
		// TODO check for unsupported flags
		binary.Read(bytes.NewBuffer(r.buf[1:5]), binary.BigEndian, &l)
		frameLen := int(l) + 5
		if len(r.buf) < 5 {
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
// must be called
func (r *responseWriter) writeTrailers() {
	if r.headerWritten || r.bodyWritten {
		// extract unwritten trailers from header
		buf := &bytes.Buffer{}
		skip := map[string]bool{"trailer": true}
		for k := range r.ResponseWriter.Header() {
			skip[strings.ToLower(k)] = true
		}
		r.header.WriteSubset(buf, skip)

		// trailerDataFrameHeader := []byte{0, 0, 0, 0, 0}
		trailerDataFrameHeader := []byte{1 << 7, 0, 0, 0, 0}
		binary.BigEndian.PutUint32(trailerDataFrameHeader[1:5], uint32(buf.Len()))
		r.buf = append(trailerDataFrameHeader, buf.Bytes()...)
		r.ResponseWriter.Write(r.encodeBase64())
		// r.ResponseWriter.Write(trailerDataFrameHeader)
		// r.ResponseWriter.Write(buf.Bytes())
	} else {
		r.WriteHeader(http.StatusOK)
	}
	r.Flush()
}

// Flush implements http.Flusher
// safe to call at any time
// must call to ensure writes happen
func (r *responseWriter) Flush() {
	if r.headerWritten || r.bodyWritten {
		if r.text {
			r.ResponseWriter.Write(r.encodeBase64())
			// internal buffer should be empty
		}
		r.ResponseWriter.(http.Flusher).Flush()
	}
}

// CloseNotify implements http.CloseNotifier
func (r *responseWriter) CloseNotify() <-chan bool {
	return r.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// Header implements http.ResponseWriter
func (r *responseWriter) Header() http.Header { return r.header }
