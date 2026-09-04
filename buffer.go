package httpproxy

import (
	"io"
	"mime"
	"net/http"
	"slices"
	"sync"

	"golang.org/x/net/http/httpguts"
)

// StreamTypes is the list of media types
// that should be proxied as stream.
var StreamTypes = []string{
	"text/event-stream",
	"application/grpc",
	"application/grpc+proto",
	"application/grpc-web+proto",
	"multipart/x-mixed-replace",
	"application/x-ndjson",
	"application/jsonlines",
	"application/x-jsonlines",
}

// pool is the buffer pool.
var pool = sync.Pool{
	New: func() any {
		buf := make([]byte, 1<<14) // 16kiB
		return &buf
	},
}

// copyBuf copies from src to dst.
// A nil or non-nil error will be sent to the errChan.
// errChan should be buffered channel to avoid dead locks.
//
//	errChan := make(chan error, 1)
func copyBuf(dst io.Writer, src io.Reader, errChan chan<- error) {
	buf := *pool.Get().(*[]byte)
	defer pool.Put(&buf)
	_, err := io.CopyBuffer(dst, src, buf)
	errChan <- err
}

// copyResponseBody copies proxy response body.
// w should be the frontend response writer and the res
// should be the response of proxy request.
func copyResponseBody(w http.ResponseWriter, res *http.Response) error {
	var dst io.Writer = w
	switch {
	case res.ContentLength < 0:
		dst = withImmediateFlushWriter(w) // Any streaming type response.
	case httpguts.HeaderValuesContainsToken(res.Header["Transfer-Encoding"], "chunked"):
		dst = withImmediateFlushWriter(w) // Chunked response.
	default:
		mt, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if slices.Contains(StreamTypes, mt) {
			dst = withImmediateFlushWriter(w) // Stream content types.
		}
	}
	buf := *pool.Get().(*[]byte)
	defer pool.Put(&buf)
	_, err := io.CopyBuffer(dst, res.Body, buf)
	return err
}

// withImmediateFlushWriter wraps the [net/http.ResponseWriter] with
// immediateFlushWriter if the rw implements `FlushError() error` or `Flush()`.
// Otherwise, it returns rw.
func withImmediateFlushWriter(rw http.ResponseWriter) io.Writer {
	fw := rw
	for {
		if flusher, ok := fw.(interface{ FlushError() error }); ok {
			return &immediateFlushWriter{
				inner: rw, // Use rw, not inner.
				flush: flusher.FlushError,
			}
		}
		if flusher, ok := fw.(interface{ Flush() error }); ok {
			return &immediateFlushWriter{
				inner: rw, // Use rw, not inner.
				flush: flusher.Flush,
			}
		}
		if flusher, ok := fw.(http.Flusher); ok {
			return &immediateFlushWriter{
				inner: rw, // Use rw, not inner.
				flush: func() error {
					flusher.Flush()
					return nil
				},
			}
		}
		if uw, ok := fw.(interface{ Unwrap() http.ResponseWriter }); ok {
			fw = uw.Unwrap()
			continue
		}
		return rw
	}
}

// immediateFlushWriter flushes immediately after Write called.
// The inner writer and flusher must not be nil.
type immediateFlushWriter struct {
	inner io.Writer
	flush func() error
}

func (f *immediateFlushWriter) Write(p []byte) (n int, err error) {
	defer func() {
		e := f.flush()
		if err == nil {
			err = e
		}
	}()
	return f.inner.Write(p)
}
