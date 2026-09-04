package httpproxy

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestHTTPHeader(t *testing.T) {
	t.Parallel()
	t.Run("add", func(t *testing.T) {
		h := httpHeader{}
		h.Add("Test", "v1")
		h.Add("Test", "v2")
		tester.AssertDeepEqual(t, []string{"v1", "v2"}, h["Test"])
	})
	t.Run("del", func(t *testing.T) {
		h := httpHeader{"Test": {"value", "value"}}
		h.Del("Test")
		tester.AssertDeepEqual(t, nil, h["Test"])
	})
	t.Run("get exist", func(t *testing.T) {
		h := httpHeader{"Test": {"v1", "v2"}}
		tester.AssertEqual(t, "v1", h.Get("Test"))
	})
	t.Run("get non exist", func(t *testing.T) {
		h := httpHeader{}
		tester.AssertEqual(t, "", h.Get("Test"))
	})
	t.Run("set", func(t *testing.T) {
		h := httpHeader{"Test": {"v1", "v2"}}
		h.Set("Test", "v3")
		tester.AssertDeepEqual(t, []string{"v3"}, h["Test"])
	})
	t.Run("values", func(t *testing.T) {
		h := httpHeader{"Test": {"v1", "v2"}}
		tester.AssertDeepEqual(t, []string{"v1", "v2"}, h.Values("Test"))
	})
}

func TestSetViaHeader(t *testing.T) {
	t.Parallel()
	t.Run("non empty proto and by", func(t *testing.T) {
		r := &http.Request{
			Host: "test.com",
		}
		h := http.Header{}
		SetViaHeader(r, h, "1.1", "test")
		tester.AssertEqual(t, `1.1 test`, h.Get("Via"))
	})
	t.Run("with prior", func(t *testing.T) {
		r := &http.Request{
			Host:   "test.com",
			Header: http.Header{"Via": []string{"1.0 pre"}},
		}
		h := http.Header{}
		SetViaHeader(r, h, "1.1", "test")
		tester.AssertEqual(t, `1.0 pre, 1.1 test`, h.Get("Via"))
	})
	t.Run("empty proto", func(t *testing.T) {
		r := &http.Request{
			Host: "test.com",
		}
		h := http.Header{}
		SetViaHeader(r, h, "", "test")
		tester.AssertEqual(t, `test`, h.Get("Via"))
	})
	t.Run("empty by", func(t *testing.T) {
		r := &http.Request{
			Host: "test.com",
		}
		h := http.Header{}
		SetViaHeader(r, h, "1.1", "")
		tester.AssertEqual(t, `1.1 go-httpproxy`, h.Get("Via"))
	})
}

func TestSetForwardedHeader(t *testing.T) {
	t.Parallel()
	t.Run("valid remote ipv4 addr", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="127.0.0.1:1234"; host="test.com"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("valid remote ipv6 addr", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "[::1]:1234",
			Host:       "test.com",
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="[::1]:1234"; host="test.com"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("invalid remote addr", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "INVALID IP",
			Host:       "test.com",
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for=unknown; host="test.com"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("ipv4 host", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "127.0.0.1:5678",
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="127.0.0.1:1234"; host="127.0.0.1:5678"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("ipv4 host", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "[::1]:5678",
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="127.0.0.1:1234"; host="[::1]:5678"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("https", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			TLS:        &tls.ConnectionState{},
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="127.0.0.1:1234"; host="test.com"; proto=https`, h.Get("Forwarded"))
	})
	t.Run("Forwarded exists", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header: http.Header{
				"Forwarded": []string{`for="192.168.0.1"`},
			},
		}
		h := http.Header{}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, `for="192.168.0.1", for="127.0.0.1:1234"; host="test.com"; proto=http`, h.Get("Forwarded"))
	})
	t.Run("don't set Forwarded", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header:     http.Header{},
		}
		h := http.Header{"Forwarded": nil}
		SetForwardedHeader(r, h)
		tester.AssertEqual(t, "", h.Get("Forwarded"))
	})
}

func TestSetXForwardedHeaders(t *testing.T) {
	t.Parallel()
	t.Run("valid remote addr", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
		}
		h := http.Header{}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "127.0.0.1", h.Get("X-Forwarded-For"))
		tester.AssertEqual(t, "1234", h.Get("X-Forwarded-Port"))
		tester.AssertEqual(t, "test.com", h.Get("X-Forwarded-Host"))
		tester.AssertEqual(t, "http", h.Get("X-Forwarded-Proto"))
	})
	t.Run("invalid remote addr", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "INVALID IP",
			Host:       "test.com",
		}
		h := http.Header{}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "", h.Get("X-Forwarded-For"))
		tester.AssertEqual(t, "", h.Get("X-Forwarded-Port"))
		tester.AssertEqual(t, "test.com", h.Get("X-Forwarded-Host"))
		tester.AssertEqual(t, "http", h.Get("X-Forwarded-Proto"))
	})
	t.Run("https", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			TLS:        &tls.ConnectionState{},
		}
		h := http.Header{}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "127.0.0.1", h.Get("X-Forwarded-For"))
		tester.AssertEqual(t, "1234", h.Get("X-Forwarded-Port"))
		tester.AssertEqual(t, "test.com", h.Get("X-Forwarded-Host"))
		tester.AssertEqual(t, "https", h.Get("X-Forwarded-Proto"))
	})
	t.Run("Forwarded exists", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header: http.Header{
				"X-Forwarded-For":   []string{"192.168.0.1"},
				"X-Forwarded-Port":  []string{"5678"},
				"X-Forwarded-Host":  []string{"prior.com"},
				"X-Forwarded-Proto": []string{"https"},
			},
		}
		h := http.Header{}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "192.168.0.1, 127.0.0.1", h.Get("X-Forwarded-For"))
		tester.AssertEqual(t, "1234", h.Get("X-Forwarded-Port"))
		tester.AssertEqual(t, "test.com", h.Get("X-Forwarded-Host"))
		tester.AssertEqual(t, "http", h.Get("X-Forwarded-Proto"))
	})
	t.Run("don't set X-Forwarded-For", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header:     http.Header{},
		}
		h := http.Header{"X-Forwarded-For": nil}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "", h.Get("X-Forwarded-For"))
	})
	t.Run("don't set X-Forwarded-Port", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header:     http.Header{},
		}
		h := http.Header{"X-Forwarded-Port": nil}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "", h.Get("X-Forwarded-Port"))
	})
	t.Run("don't set X-Forwarded-Host", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header:     http.Header{},
		}
		h := http.Header{"X-Forwarded-Host": nil}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "", h.Get("X-Forwarded-Host"))
	})
	t.Run("don't set X-Forwarded-Proto", func(t *testing.T) {
		r := &http.Request{
			RemoteAddr: "127.0.0.1:1234",
			Host:       "test.com",
			Header:     http.Header{},
		}
		h := http.Header{"X-Forwarded-Proto": nil}
		SetXForwardedHeaders(r, h)
		tester.AssertEqual(t, "", h.Get("X-Forwarded-Proto"))
	})
}

func TestRemoveHopByHopHeaders(t *testing.T) {
	t.Parallel()
	h := http.Header{
		"Connection":          []string{"foo, bar"},
		"Keep-Alive":          []string{"Value-Keep-Alive"},
		"Proxy-Authenticate":  []string{"Value-Proxy-Authenticate"},
		"Proxy-Authorization": []string{"Value-Proxy-Authorization"},
		"Te":                  []string{"Value-Te"},
		"Trailer":             []string{"Value-Trailer"},
		"Transfer-Encoding":   []string{"Value-Transfer-Encoding"},
		"Upgrade":             []string{"Value-Upgrade"},
		"Proxy-Connection":    []string{"Value-Proxy-Connection"},
		"Foo":                 []string{"Value-Foo"},
		"Bar":                 []string{"Value-Bar"},
		"Baz":                 []string{"Value-Baz"},
	}
	removeHopByHopHeaders(h)
	tester.AssertEqual(t, "", h.Get("Keep-Alive"))
	tester.AssertEqual(t, "", h.Get("Proxy-Authenticate"))
	tester.AssertEqual(t, "", h.Get("Proxy-Authorization"))
	tester.AssertEqual(t, "", h.Get("Te"))
	tester.AssertEqual(t, "", h.Get("Trailer"))
	tester.AssertEqual(t, "", h.Get("Transfer-Encoding"))
	tester.AssertEqual(t, "", h.Get("Upgrade"))
	tester.AssertEqual(t, "", h.Get("Proxy-Connection"))
	tester.AssertEqual(t, "", h.Get("Foo"))
	tester.AssertEqual(t, "", h.Get("Bar"))
	tester.AssertEqual(t, "Value-Baz", h.Get("Baz"))
}

func TestCopyHeaders(t *testing.T) {
	t.Parallel()
	dst := http.Header{
		"Foo": []string{"foo"},
		"Bar": []string{"bar1"},
	}
	src := http.Header{
		"Bar": []string{"bar2"},
		"Baz": []string{"baz"},
	}
	copyHeaders(dst, src)
	tester.AssertDeepEqual(t, []string{"foo"}, dst.Values("Foo"))
	tester.AssertDeepEqual(t, []string{"bar1", "bar2"}, dst.Values("Bar"))
	tester.AssertDeepEqual(t, []string{"baz"}, dst.Values("Baz"))
}

func TestCopyTrailers(t *testing.T) {
	t.Parallel()
	dst := http.Header{
		"Foo":                      []string{"foo"},
		"Bar":                      []string{"bar1"},
		http.TrailerPrefix + "Bar": []string{"bar1"},
	}
	src := http.Header{
		"Bar": []string{"bar2"},
		"Baz": []string{"baz"},
	}
	copyTrailers(dst, src)
	tester.AssertDeepEqual(t, []string{"foo"}, dst.Values("Foo"))
	tester.AssertDeepEqual(t, []string{"bar1"}, dst.Values("Bar"))
	tester.AssertDeepEqual(t, []string{"bar1", "bar2"}, dst.Values(http.TrailerPrefix+"Bar"))
	tester.AssertDeepEqual(t, []string{"baz"}, dst.Values(http.TrailerPrefix+"Baz"))
}

func TestUpgradeType(t *testing.T) {
	t.Parallel()
	t.Run("no header", func(t *testing.T) {
		h := http.Header{}
		s := upgradeType(h)
		tester.AssertEqual(t, "", s)
	})
	t.Run("with header no value", func(t *testing.T) {
		h := http.Header{"Connection": []string{"upgrade"}}
		s := upgradeType(h)
		tester.AssertEqual(t, "", s)
	})
	t.Run("with header", func(t *testing.T) {
		h := http.Header{"Connection": []string{"upgrade"}, "Upgrade": []string{"test"}}
		s := upgradeType(h)
		tester.AssertEqual(t, "test", s)
	})
}
