package httpproxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestCopyBuf(t *testing.T) {
	t.Parallel()
	t.Run("no error", func(t *testing.T) {
		var dst bytes.Buffer
		src := strings.NewReader("test")
		errChan := make(chan error, 1)
		copyBuf(&dst, src, errChan)
		tester.AssertEqual(t, "test", dst.String())
		tester.AssertEqual(t, nil, <-errChan)
	})
	t.Run("read error", func(t *testing.T) {
		err := errors.New("read error")
		var dst bytes.Buffer
		src := tester.NewMaxReader(strings.NewReader("test"), 3, err)
		errChan := make(chan error, 1)
		copyBuf(&dst, src, errChan)
		tester.AssertEqual(t, "tes", dst.String())
		tester.AssertEqual(t, err, <-errChan)
	})
	t.Run("write error", func(t *testing.T) {
		err := errors.New("wite error")
		dst := tester.NewMaxWriter(3, err)
		src := strings.NewReader("test")
		errChan := make(chan error, 1)
		copyBuf(dst, src, errChan)
		tester.AssertEqual(t, "tes", dst.String())
		tester.AssertEqual(t, err, <-errChan)
	})
}

type testResponse struct {
	http.ResponseWriter
	body    bytes.Buffer
	flushed bool
}

func (r *testResponse) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func (r *testResponse) Flush() {
	r.flushed = true
}

func TestCopyResponseBody(t *testing.T) {
	t.Parallel()
	newTestBody := func() io.ReadCloser {
		return io.NopCloser(strings.NewReader("test"))
	}
	t.Run("stream response", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &testResponse{ResponseWriter: rec}
		res := &http.Response{ContentLength: -1, Body: newTestBody()}
		copyResponseBody(w, res)
		tester.AssertEqual(t, true, w.flushed)
		tester.AssertEqual(t, "test", w.body.String())
	})
	t.Run("sse", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &testResponse{ResponseWriter: rec}
		res := &http.Response{ContentLength: 0,
			Header: http.Header{"Content-Type": {"text/event-stream"}},
			Body:   newTestBody(),
		}
		copyResponseBody(w, res)
		tester.AssertEqual(t, true, w.flushed)
		tester.AssertEqual(t, "test", w.body.String())
	})
	t.Run("chunked", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &testResponse{ResponseWriter: rec}
		res := &http.Response{ContentLength: 0,
			Header: http.Header{"Transfer-Encoding": {"chunked"}},
			Body:   newTestBody(),
		}
		copyResponseBody(w, res)
		tester.AssertEqual(t, true, w.flushed)
		tester.AssertEqual(t, "test", w.body.String())
	})
	t.Run("default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &testResponse{ResponseWriter: rec}
		res := &http.Response{Body: newTestBody()}
		copyResponseBody(w, res)
		tester.AssertEqual(t, false, w.flushed)
		tester.AssertEqual(t, "test", w.body.String())
	})
}

type testFlushErrorResponse struct {
	http.ResponseWriter
	body    bytes.Buffer
	err     error
	flushed bool
}

func (r *testFlushErrorResponse) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func (r *testFlushErrorResponse) FlushError() error {
	r.flushed = true
	return r.err
}

type testFlushResponse struct {
	http.ResponseWriter
	body    bytes.Buffer
	err     error
	flushed bool
}

func (r *testFlushResponse) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func (r *testFlushResponse) Flush() error {
	r.flushed = true
	return r.err
}

type testResponseWrapper struct {
	http.ResponseWriter
	body bytes.Buffer
}

func (r *testResponseWrapper) Write(p []byte) (int, error) {
	r.body.Write(p)
	return r.ResponseWriter.Write(p)
}

func (r *testResponseWrapper) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func TestWithImmediateFlushWriter(t *testing.T) {
	t.Parallel()
	t.Run("no flush", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := withImmediateFlushWriter(rec)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("Flush", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testResponse{ResponseWriter: rec}
		w := withImmediateFlushWriter(rw)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("FlushError nil error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testFlushErrorResponse{ResponseWriter: rec}
		w := withImmediateFlushWriter(rw)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("FlushError non-nil error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testFlushErrorResponse{ResponseWriter: rec, err: io.EOF}
		w := withImmediateFlushWriter(rw)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, io.EOF, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("Flush nil error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testFlushResponse{ResponseWriter: rec}
		w := withImmediateFlushWriter(rw)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("Flush non-nil error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testFlushResponse{ResponseWriter: rec, err: io.EOF}
		w := withImmediateFlushWriter(rw)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, io.EOF, err)
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("unwrap flush", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &testResponse{ResponseWriter: rec}
		tr := &testResponseWrapper{ResponseWriter: rw}
		w := withImmediateFlushWriter(tr)
		_, err := w.Write([]byte("test"))
		tester.AssertEqual(t, true, rw.flushed)
		tester.AssertEqual(t, nil, err)
		tester.AssertEqual(t, "test", tr.body.String())
		tester.AssertEqual(t, "test", rec.Body.String())
	})
	t.Run("no flusher", func(t *testing.T) {
		tr := &testResponseWrapper{ResponseWriter: nil}
		w := withImmediateFlushWriter(tr)
		tester.AssertEqual(t, io.Writer(tr), w)
	})
}
