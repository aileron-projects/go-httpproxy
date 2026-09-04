package httpproxy

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/aileron-projects/go-tester"
)

type nopTransport struct {
	r *http.Request
}

func (t *nopTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.r = r
	return nil, errors.New("roundtrip error")
}

func TestNewProxy(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		targets []string
		err     error
	}{
		"single":   {targets: []string{"http://test.com"}, err: nil},
		"multiple": {targets: []string{"http://test1.com", "http://test2.com"}, err: nil},
		"error":    {targets: []string{"%http://test.com"}, err: &url.Error{Op: "parse", URL: "%http://test.com"}},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			proxy, err := NewProxy(tc.targets...)
			if tc.err != nil {
				tester.AssertEqual(t, true, err != nil)
				return
			}
			tester.AssertEqualErr(t, nil, err)
			nt := &nopTransport{}
			proxy.Transport = nt
			r := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
			w := httptest.NewRecorder()
			for i := range 2 * len(tc.targets) {
				target := tc.targets[i%len(tc.targets)]
				proxy.ServeHTTP(w, r)
				tester.AssertEqual(t, target, nt.r.URL.String())
			}
		})
	}
}
func TestRewriteProxyURL(t *testing.T) {
	t.Parallel()
	parse := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			panic(err)
		}
		return u
	}
	testCases := map[string]struct {
		dst, src *url.URL
		want     *url.URL
	}{
		"case1":            {&url.URL{}, parse("example.com"), &url.URL{Path: "example.com"}},
		"case2":            {&url.URL{}, parse("http://example.com"), &url.URL{Scheme: "http", Host: "example.com"}},
		"case3":            {&url.URL{}, parse("http://example.com/test"), &url.URL{Scheme: "http", Host: "example.com", Path: "/test"}},
		"rewrite schema":   {parse("https://example.com"), parse("http://example.com"), &url.URL{Scheme: "http", Host: "example.com"}},
		"rewrite host":     {parse("http://foo.com"), parse("http://bar.com"), &url.URL{Scheme: "http", Host: "bar.com"}},
		"rewrite path":     {parse("http://example.com/foo"), parse("http://example.com/bar"), &url.URL{Scheme: "http", Host: "example.com", Path: "/bar/foo"}},
		"rewrite query":    {parse("http://example.com?foo=alice"), parse("http://example.com?bar=bob"), &url.URL{Scheme: "http", Host: "example.com", RawQuery: "bar=bob&foo=alice"}},
		"rewrite fragment": {parse("http://example.com#foo"), parse("http://example.com#bar"), &url.URL{Scheme: "http", Host: "example.com", Fragment: "bar"}},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			RewriteURL(tc.dst, tc.src)
			tester.AssertDeepEqual(t, tc.want, tc.dst)
		})
	}
}

func TestJoinWithByte(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		left, right string
		want        string
	}{
		"case01": {"", "", ""},
		"case02": {"foo", "", "foo"},
		"case03": {"/foo", "", "/foo"},
		"case04": {"foo/", "", "foo/"},
		"case05": {"/foo/", "", "/foo/"},
		"case06": {"", "bar", "bar"},
		"case07": {"foo", "bar", "foo/bar"},
		"case08": {"foo", "/bar", "foo/bar"},
		"case09": {"foo", "bar/", "foo/bar/"},
		"case10": {"foo", "/bar/", "foo/bar/"},
		"case11": {"/foo", "bar", "/foo/bar"},
		"case12": {"/foo", "/bar", "/foo/bar"},
		"case13": {"/foo", "bar/", "/foo/bar/"},
		"case14": {"/foo", "/bar/", "/foo/bar/"},
		"case15": {"foo/", "bar", "foo/bar"},
		"case16": {"foo/", "/bar", "foo/bar"},
		"case17": {"foo/", "bar/", "foo/bar/"},
		"case18": {"foo/", "/bar/", "foo/bar/"},
		"case19": {"/foo/", "bar", "/foo/bar"},
		"case20": {"/foo/", "/bar", "/foo/bar"},
		"case21": {"/foo/", "bar/", "/foo/bar/"},
		"case22": {"/foo/", "/bar/", "/foo/bar/"},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := joinWithByte(tc.left, tc.right, '/')
			tester.AssertEqual(t, tc.want, got)
		})
	}
}
