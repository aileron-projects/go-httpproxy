package httpproxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aileron-projects/go-httpproxy"
)

type mockTransport struct {
	http.RoundTripper
}

func (t *mockTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Benchmark": []string{"ok"},
			"Foo":       []string{"foo1", "foo2", "foo3", "foo4", "foo5"},
			"Bar":       []string{"bar1", "bar2", "bar3", "bar4", "bar5"},
			"Baz":       []string{"baz1", "baz2", "baz3", "baz4", "baz5"},
		},
		Body: io.NopCloser(strings.NewReader(strings.Repeat("1234567890", 100))),
	}, nil
}

func BenchmarkProxy(b *testing.B) {
	proxy, _ := httpproxy.NewProxy("localhost:9999")
	proxy.Transport = &mockTransport{}
	b.ResetTimer()
	for b.Loop() {
		r := httptest.NewRequest(http.MethodGet, "http://localhost:12345/foo/bar/baz?x=alice&y=bob&z=charlie", http.NoBody)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, r)
		if w.Result().Header.Get("Benchmark") != "ok" {
			b.Error("check header does not exist")
		}
	}
}
