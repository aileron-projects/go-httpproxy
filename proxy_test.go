package httpproxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aileron-projects/go-httpproxy"
	"github.com/aileron-projects/go-tester"
)

type testTransport struct {
	req  *http.Request
	resp *http.Response
	err  error
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.req = req
	return t.resp, t.err
}

type checkClosedReadCloser struct {
	io.Reader
	closed bool
}

func (c *checkClosedReadCloser) Close() error {
	c.closed = true
	return nil
}

func TestProxy(t *testing.T) {
	t.Parallel()
	t.Run("simple request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Test": {"foo"}},
				Body:       io.NopCloser(strings.NewReader("resp body")),
			},
		}
		proxy := &httpproxy.Proxy{
			Transport: tp,
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, http.MethodGet, tp.req.Method)
		tester.AssertEqual(t, "http://test.com", tp.req.URL.String())
		tester.AssertEqual(t, http.StatusBadRequest, resp.Result().StatusCode)
		tester.AssertEqual(t, "foo", resp.Header().Get("Test"))
		tester.AssertEqual(t, "resp body", resp.Body.String())
	})
	t.Run("request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "http://test.com", strings.NewReader("req body"))
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{},
				Body:       http.NoBody,
			},
		}
		proxy := &httpproxy.Proxy{
			Transport: tp,
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, http.MethodPost, tp.req.Method)
		b, _ := io.ReadAll(tp.req.Body)
		tester.AssertEqual(t, "req body", string(b))
	})
	t.Run("pre roundtrip", func(t *testing.T) {
		bodyBefore := &checkClosedReadCloser{Reader: strings.NewReader("req body")}
		bodyAfter := &checkClosedReadCloser{Reader: strings.NewReader("replaced body")}
		req := httptest.NewRequest(http.MethodPost, "http://test.com", bodyBefore)
		req.Header.Set("Test", "foo")
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{},
				Body:       http.NoBody,
			},
		}
		proxy := &httpproxy.Proxy{
			PreRoundTrip: func(pr *httpproxy.ProxyRequest) error {
				pr.Out.Header.Set("Test", "bar")
				pr.Out.Body = bodyAfter
				return nil
			},
			Transport: tp,
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, "bar", tp.req.Header.Get("Test"))
		b, _ := io.ReadAll(tp.req.Body)
		tester.AssertEqual(t, "replaced body", string(b))
		tester.AssertEqual(t, true, bodyBefore.closed)
		tester.AssertEqual(t, true, bodyAfter.closed)
	})
	t.Run("post roundtrip", func(t *testing.T) {
		bodyBefore := &checkClosedReadCloser{Reader: strings.NewReader("resp body")}
		bodyAfter := &checkClosedReadCloser{Reader: strings.NewReader("replaced body")}
		req := httptest.NewRequest(http.MethodPost, "http://test.com", strings.NewReader("req body"))
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Test": []string{"foo"}},
				Body:       bodyBefore,
			},
		}
		proxy := &httpproxy.Proxy{
			PostRoundTrip: func(in *httpproxy.In, out *httpproxy.Out) error {
				out.Res.Header.Set("Test", "bar")
				out.Res.Body = bodyAfter
				return nil
			},
			Transport: tp,
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, "bar", resp.Result().Header.Get("Test"))
		b, _ := io.ReadAll(resp.Result().Body)
		tester.AssertEqual(t, "replaced body", string(b))
		tester.AssertEqual(t, true, bodyBefore.closed)
		tester.AssertEqual(t, true, bodyAfter.closed)
	})
	t.Run("pre roundtrip error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
			},
		}
		var got *httpproxy.Error
		proxy := &httpproxy.Proxy{
			Transport: tp,
			PreRoundTrip: func(pr *httpproxy.ProxyRequest) error {
				return io.ErrUnexpectedEOF
			},
			ErrorHandler: func(_ http.ResponseWriter, _ *http.Request, err *httpproxy.Error) {
				got = err
			},
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, io.ErrUnexpectedEOF, got.Inner)
		tester.AssertEqual(t, httpproxy.ErrPreRoundTrip, got.Cause)
	})
	t.Run("post roundtrip error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       http.NoBody,
			},
		}
		var got *httpproxy.Error
		proxy := &httpproxy.Proxy{
			Transport: tp,
			PostRoundTrip: func(i *httpproxy.In, o *httpproxy.Out) error {
				return io.ErrUnexpectedEOF

			},
			ErrorHandler: func(_ http.ResponseWriter, _ *http.Request, err *httpproxy.Error) {
				got = err
			},
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, io.ErrUnexpectedEOF, got.Inner)
		tester.AssertEqual(t, httpproxy.ErrPostRoundTrip, got.Cause)
	})
	t.Run("upgrade error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		req.Header.Set("Connection", "upgrade")
		req.Header.Set("Upgrade", "test-protocol")
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusSwitchingProtocols,
				Header:     http.Header{"Connection": []string{"upgrade"}, "Upgrade": []string{"test-protocol"}},
				Body:       http.NoBody, // Cause an error.
			},
		}
		var got *httpproxy.Error
		proxy := &httpproxy.Proxy{
			Transport: tp,
			ErrorHandler: func(_ http.ResponseWriter, _ *http.Request, err *httpproxy.Error) {
				got = err
			},
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, httpproxy.ErrNonSwitchable, got.Cause)
		tester.AssertEqual(t, true, got.Writable())
	})
	t.Run("send response error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://test.com", nil)
		tp := &testTransport{
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Test": []string{"foo"}},
				Body:       io.NopCloser(tester.NewMaxReader(strings.NewReader("resp body"), 4, tester.ErrMaxRead)),
			},
		}
		var got *httpproxy.Error
		proxy := &httpproxy.Proxy{
			Transport: tp,
			ErrorHandler: func(_ http.ResponseWriter, _ *http.Request, err *httpproxy.Error) {
				got = err
			},
		}
		resp := httptest.NewRecorder()
		proxy.ServeHTTP(resp, req)
		tester.AssertEqual(t, http.StatusOK, resp.Result().StatusCode)
		tester.AssertEqual(t, "foo", resp.Header().Get("Test"))
		tester.AssertEqual(t, "resp", resp.Body.String())
		tester.AssertEqual(t, httpproxy.ErrWriteResponse, got.Cause)
		tester.AssertEqual(t, false, got.Writable())
	})
}
