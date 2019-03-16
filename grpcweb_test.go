package grpcweb

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

var (
	testHandler = &TestH{}
	testRW      = &TestRw{}
)

// test handler is wrapped
func TestNew(t *testing.T) {
	tests := []string{
		"hello world",
	}

	for i, test := range tests {
		// setup
		h := New(testHandler)
		r := &TestRw{}
		req, _ := http.NewRequest(http.MethodGet, "/", strings.NewReader(test))

		// run
		h.ServeHTTP(r, req)

		// validate
		if string(r.b) != test {
			t.Errorf("TestNew %d: expected %s, got %s\n", i, test, string(r.b))
		}
	}

}

//  test requests are treated properly
// TODO add test cases:
//	http1 -> http2
//	content-length stripped
//	content-type
//	body decoded
func TestRequest(t *testing.T) {
	tests := []struct {
		input, expected *http.Request
	}{}

	for i, test := range tests {
		// setup
		h := New(testHandler)
		r := &TestRw{}

		// run
		h.ServeHTTP(r, test.input)

		// validate
		if true {
			t.Errorf("TestRequest %d: expected %v got %v\n", i, test.expected, r.r)
		}
	}
}

//
// // test reponses are treated properly
// func TestResponseGRPCWeb(t *testing.T)     {}
// func TestResponseGRPCWebText(t *testing.T) {}

// ========================= test utils

// TestRw is a test util ResponseWriter
// logs everything
type TestRw struct {
	b []byte
	r *http.Request
}

func (r *TestRw) Header() http.Header         { return make(http.Header) }
func (r *TestRw) Write(p []byte) (int, error) { r.b = append(r.b, p...); return len(p), nil }
func (r *TestRw) WriteHeader(s int)           {}

// TestH is a test util Handler
type TestH struct {
}

func (h *TestH) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	w.Write(b)
}
