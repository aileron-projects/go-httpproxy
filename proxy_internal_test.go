package httpproxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestHandleError(t *testing.T) {
	t.Parallel()
	t.Run("no custom handler", func(t *testing.T) {
		p := &Proxy{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		err := &Error{}
		p.handleError(w, r, err)
		tester.AssertEqual(t, http.StatusBadGateway, w.Result().StatusCode)
		tester.AssertEqual(t, http.StatusText(http.StatusBadGateway), w.Body.String())
	})
	t.Run("already written", func(t *testing.T) {
		p := &Proxy{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		err := &Error{written: true}
		p.handleError(w, r, err)
		tester.AssertEqual(t, http.StatusOK, w.Result().StatusCode)
		tester.AssertEqual(t, "", w.Body.String())
	})
	t.Run("custom handler", func(t *testing.T) {
		var got *Error
		p := &Proxy{
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err *Error) {
				got = err
			},
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		err := &Error{written: false}
		p.handleError(w, r, err)
		tester.AssertEqual(t, err, got)
	})
	t.Run("request is not canceled", func(t *testing.T) {
		var got *Error
		p := &Proxy{
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err *Error) {
				got = err
			},
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		err := &Error{Cause: ErrRoundTrip, written: false}
		p.handleError(w, r, err)
		tester.AssertEqualErr(t, &Error{Cause: ErrRoundTrip}, got)
		tester.AssertEqual(t, true, err.Writable())
	})
	t.Run("request was canceled", func(t *testing.T) {
		var got *Error
		p := &Proxy{
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err *Error) {
				got = err
			},
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r = r.WithContext(ctx)
		err := &Error{Cause: ErrRoundTrip, written: false}
		p.handleError(w, r, err)
		tester.AssertEqualErr(t, &Error{Cause: ErrClientCanceled}, got)
		tester.AssertEqual(t, false, err.Writable())
	})
	t.Run("request was canceled with cause", func(t *testing.T) {
		var got *Error
		p := &Proxy{
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err *Error) {
				got = err
			},
		}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(io.EOF)
		r = r.WithContext(ctx)
		err := &Error{Cause: ErrRoundTrip, written: true}
		p.handleError(w, r, err)
		tester.AssertEqualErr(t, &Error{Cause: ErrRoundTrip}, got)
		tester.AssertEqual(t, false, err.Writable())
	})
}

func TestNewProxyRequest(t *testing.T) {
	t.Parallel()
	t.Run("simple request", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", nil)
		got := newProxyRequest(r)
		wantURL, _ := url.Parse("http://localhost:99999/test")
		wantHeader := http.Header{}
		wantHeader.Set("User-Agent", "")
		tester.AssertEqual(t, r.Context(), got.Context())
		tester.AssertDeepEqual(t, wantURL, got.URL)
		tester.AssertDeepEqual(t, wantHeader, got.Header)
		tester.AssertEqual(t, nil, got.Body)
	})
	t.Run("nil header", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", nil)
		r.Header = nil
		got := newProxyRequest(r)
		wantHeader := http.Header{}
		wantHeader.Set("User-Agent", "")
		tester.AssertDeepEqual(t, wantHeader, got.Header)
	})
	t.Run("non-nil body", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", strings.NewReader("test"))
		r.Header = nil
		got := newProxyRequest(r)
		b, _ := io.ReadAll(got.Body)
		tester.AssertEqual(t, "test", string(b))
	})
	t.Run("use Use-Agent", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", nil)
		r.Header.Set("User-Agent", "test")
		got := newProxyRequest(r)
		wantHeader := http.Header{"User-Agent": []string{"test"}}
		tester.AssertDeepEqual(t, wantHeader, got.Header)
	})
	t.Run("use trailers", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", nil)
		r.Header.Set("Te", "trailers")
		got := newProxyRequest(r)
		wantHeader := http.Header{"Te": []string{"trailers"}}
		wantHeader.Set("User-Agent", "")
		tester.AssertDeepEqual(t, wantHeader, got.Header)
	})
	t.Run("use upgrade", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "http://localhost:99999/test", nil)
		r.Header.Set("Connection", "upgrade")
		r.Header.Set("Upgrade", "websocket")
		r.Header.Set("User-Agent", "test")
		got := newProxyRequest(r)
		wantHeader := http.Header{}
		wantHeader.Set("Connection", "upgrade")
		wantHeader.Set("Upgrade", "websocket")
		wantHeader.Set("User-Agent", "test")
		tester.AssertDeepEqual(t, wantHeader, got.Header)
	})
}

type nonFlushResponse struct {
	http.ResponseWriter
}

func TestSendResponse(t *testing.T) {
	t.Parallel()
	t.Run("simple response", func(t *testing.T) {
		w := httptest.NewRecorder()
		out := &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Foo": []string{"foo"},
				"Bar": []string{"bar"},
			},
			Body: io.NopCloser(bytes.NewReader(nil)),
		}
		err := sendResponse(w, out)
		tester.AssertEqualErr(t, (*Error)(nil), err)
		tester.AssertEqual(t, http.StatusBadRequest, w.Result().StatusCode)
		wantHeader := http.Header{}
		wantHeader.Set("Foo", "foo")
		wantHeader.Set("Bar", "bar")
		tester.AssertDeepEqual(t, wantHeader, w.Result().Header)
	})
	t.Run("response body", func(t *testing.T) {
		w := httptest.NewRecorder()
		out := &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{},
			Body:          io.NopCloser(strings.NewReader("test")),
			ContentLength: 4,
		}
		err := sendResponse(w, out)
		tester.AssertEqualErr(t, (*Error)(nil), err)
		tester.AssertEqual(t, http.StatusOK, w.Result().StatusCode)
		wantHeader := http.Header{}
		tester.AssertDeepEqual(t, wantHeader, w.Result().Header)
		b, _ := io.ReadAll(w.Result().Body)
		tester.AssertEqual(t, "test", string(b))
	})
	t.Run("trailer", func(t *testing.T) {
		w := httptest.NewRecorder()
		out := &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Trailer:    http.Header{"Foo": []string{"foo"}},
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}
		err := sendResponse(w, out)
		tester.AssertEqualErr(t, (*Error)(nil), err)
		wantHeader := http.Header{}
		wantHeader.Set("Trailer", "Foo")
		wantTrailer := http.Header{}
		wantTrailer.Set("Foo", "foo")
		tester.AssertDeepEqual(t, wantHeader, w.Result().Header)
		tester.AssertDeepEqual(t, wantTrailer, w.Result().Trailer)
	})
	t.Run("copy error", func(t *testing.T) {
		w := httptest.NewRecorder()
		out := &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Test": []string{"test"}},
			Body:          io.NopCloser(tester.NewMaxReader(strings.NewReader("test"), 3, tester.ErrMaxRead)),
			ContentLength: 4,
		}
		err := sendResponse(w, out)
		tester.AssertEqualErr(t, &Error{Cause: ErrWriteResponse}, err)
		tester.AssertEqual(t, http.StatusOK, w.Result().StatusCode)
		wantHeader := http.Header{"Test": []string{"test"}}
		tester.AssertDeepEqual(t, wantHeader, w.Result().Header)
		b, _ := io.ReadAll(w.Result().Body)
		tester.AssertEqual(t, "tes", string(b))
	})
	t.Run("flush error", func(t *testing.T) {
		w := httptest.NewRecorder()
		out := &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Foo": []string{"foo"}},
			Trailer:       http.Header{"Bar": []string{"bar"}},
			Body:          io.NopCloser(strings.NewReader("test")),
			ContentLength: 4,
		}
		err := sendResponse(&nonFlushResponse{w}, out)
		tester.AssertEqualErr(t, &Error{Cause: ErrFlushResponse}, err)
		tester.AssertEqual(t, http.StatusOK, w.Result().StatusCode)
		wantHeader := http.Header{}
		wantHeader.Set("Foo", "foo")
		wantHeader.Set("Trailer", "Bar")
		wantTrailer := http.Header{}
		tester.AssertDeepEqual(t, wantHeader, w.Result().Header)
		tester.AssertDeepEqual(t, wantTrailer, w.Result().Trailer)
		b, _ := io.ReadAll(w.Result().Body)
		tester.AssertEqual(t, "test", string(b))
	})
}

type testConn struct {
	io.Reader
	net.Conn
	closeErr error

	closed  bool
	content bytes.Buffer
}

func (c *testConn) Read(b []byte) (n int, err error) {
	return c.Reader.Read(b)
}

func (c *testConn) Write(b []byte) (n int, err error) {
	return c.content.Write(b)
}

func (c *testConn) Close() error {
	c.closed = true
	return c.closeErr
}

type testHijackResponse struct {
	http.ResponseWriter
	conn      net.Conn
	rw        *bufio.ReadWriter
	hijackErr error
}

func (r *testHijackResponse) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.conn, r.rw, r.hijackErr
}

type readWriteCloser struct {
	io.Reader
	io.Writer
}

func (rwc *readWriteCloser) Close() error {
	return nil
}

func TestHandleUpgradeResponse(t *testing.T) {
	t.Parallel()
	t.Run("upgrade success", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, (*Error)(nil), err)
		tester.AssertEqual(t, "foo", buf.String())
		tester.AssertEqual(t, "bar", conn.content.String())
	})
	t.Run("upgrade not match", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test1"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test2"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrProtocolMismatch}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("no connection header", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {""}, "Upgrade": {"test2"}}, Body: body} // No connection header.

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrProtocolMismatch}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("non ReadWriteCloser body", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: nil} // Body is nil.

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrNonSwitchable}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("hijack failed", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		hijackErr := errors.New("hijack failed")
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW, hijackErr: hijackErr} // Hijack error.

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrNonSwitchable}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("flush error", func(t *testing.T) {
		ew := tester.NewMaxWriter(2, tester.ErrMaxWritten) // Error writer results in flush error.
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(ew))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrFlushResponse}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("response header write error", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriterSize(tester.NewMaxWriter(0, tester.ErrMaxWritten), 1)) // Error writer.
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrWriteResponse}, err)
		tester.AssertEqual(t, "", buf.String())
		tester.AssertEqual(t, "", conn.content.String())
	})
	t.Run("copy front to back error", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: tester.NewMaxReader(strings.NewReader("foo"), 1, tester.ErrMaxRead)} // Error reader.
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: strings.NewReader("bar"), Writer: &buf}
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		tester.AssertEqualErr(t, &Error{Cause: ErrBidirectionalCom}, err)
		tester.AssertEqual(t, "f", buf.String())
		tester.AssertEqual(t, "bar", conn.content.String())
	})
	t.Run("copy back to front error", func(t *testing.T) {
		respRW := bufio.NewReadWriter(nil, bufio.NewWriter(&bytes.Buffer{}))
		conn := &testConn{Reader: strings.NewReader("foo")}
		rw := &testHijackResponse{ResponseWriter: httptest.NewRecorder(), conn: conn, rw: respRW}

		req := &http.Request{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}}
		var buf bytes.Buffer
		body := &readWriteCloser{Reader: tester.NewMaxReader(strings.NewReader("bar"), 1, tester.ErrMaxRead), Writer: &buf} // Error reader.
		res := &http.Response{Header: http.Header{"Connection": {"Upgrade"}, "Upgrade": {"test"}}, Body: body}

		err := handleUpgradeResponse(rw, req, res)
		_ = err
		tester.AssertEqualErr(t, &Error{Cause: ErrBidirectionalCom}, err)
		tester.AssertEqual(t, "foo", buf.String())
		tester.AssertEqual(t, "b", conn.content.String())
	})
}
