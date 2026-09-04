package httpproxy

import (
	"cmp"
	"net"
	"net/http"
	"net/textproto"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// httpHeader is the http header.
// Unlike [http.Header], operations for headers
// do not check if headers are nil or not
// and does not apply [http.CanonicalHeaderKey] for keys.
// Caller must ensure that the given keys are in canonical form.
// Use this where performance is important.
type httpHeader http.Header

func (h httpHeader) Add(key string, value string) {
	h[key] = append(h[key], value)
}

func (h httpHeader) Del(key string) {
	delete(h, key)
}

func (h httpHeader) Get(key string) string {
	v := h[key]
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func (h httpHeader) Set(key string, value string) {
	h[key] = []string{value}
}

func (h httpHeader) Values(key string) []string {
	return h[key]
}

// SetViaHeader sets the Via header to the h.
// Via header is defined in RFC9110.
// proto and by expect the following format.
// If the given by is empty, default "go-httpproxy" is used.
// Setting h["Via"]=nil prevents SetViaHeader from populating the Via header.
//
//	proto = received-protocol = [ protocol-name "/" ] protocol-version
//	by = received-by = pseudonym [ ":" port ]
//
// References:
//   - https://datatracker.ietf.org/doc/rfc9110/
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Via
func SetViaHeader(r *http.Request, h http.Header, proto, by string) {
	via := cmp.Or(by, "go-httpproxy")
	if proto != "" {
		via = proto + " " + via
	}
	if prior := r.Header["Via"]; len(prior) > 0 {
		via = strings.Join(prior, ", ") + ", " + via
	}
	setHeader(h, "Via", via)
}

// SetForwardedHeader sets the Forwarded header to the h.
// Forwarded header is defined in RFC7239.
// Setting h["Forwarded"]=nil prevents SetForwardedHeader from
// populating the Forwarded header.
//
// References:
//   - https://go.dev/src/net/http/httputil/reverseproxy.go
//   - https://datatracker.ietf.org/doc/rfc7239/
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded
func SetForwardedHeader(r *http.Request, h http.Header) {
	var forwarded string
	_, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		forwarded += "for=\"" + r.RemoteAddr + "\""
	} else {
		forwarded += "for=unknown"
	}

	forwarded += "; host=\"" + r.Host + "\""

	if r.TLS == nil {
		forwarded += "; proto=http"
	} else {
		forwarded += "; proto=https"
	}

	if prior := r.Header["Forwarded"]; len(prior) > 0 {
		forwarded = strings.Join(prior, ", ") + ", " + forwarded
	}
	setHeader(h, "Forwarded", forwarded)
}

// SetXForwardedHeaders sets the X-Forwarded-For, X-Forwarded-Host,
// X-Forwarded-Port and X-Forwarded-Proto headers to the given h.
// Setting h["X-Forwarded-XXX"]=nil prevents SetXForwardedHeaders from
// populating the X-Forwarded-XXX header.
//
// References:
//   - https://go.dev/src/net/http/httputil/reverseproxy.go
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Host
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-Proto
func SetXForwardedHeaders(r *http.Request, h http.Header) {
	ip, port, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if prior := r.Header["X-Forwarded-For"]; len(prior) > 0 {
			ip = strings.Join(prior, ", ") + ", " + ip
		}
		setHeader(h, "X-Forwarded-For", ip)
		setHeader(h, "X-Forwarded-Port", port)
	} else {
		h.Del("X-Forwarded-For")
		h.Del("X-Forwarded-Port")
	}

	setHeader(h, "X-Forwarded-Host", r.Host)

	setHeader(h, "X-Forwarded-Proto", "http")
	if r.TLS == nil {
		setHeader(h, "X-Forwarded-Proto", "http")
	} else {
		setHeader(h, "X-Forwarded-Proto", "https")
	}
}

func setHeader(h http.Header, key, value string) {
	prior, found := h[key]
	omit := found && prior == nil // nil now means don't populate the header
	if !omit {
		h[key] = []string{value}
	}
}

// removeHopByHopHeaders removes hop-by-hop headers.
// See the references about removed headers.
//
// References:
//   - https://go.dev/src/net/http/httputil/reverseproxy.go
//   - https://datatracker.ietf.org/doc/rfc7230/
//   - https://datatracker.ietf.org/doc/rfc2616/
//
// Removed headers:
//   - Connection
//   - Keep-Alive
//   - Proxy-Authenticate
//   - Proxy-Authorization
//   - Te
//   - Trailer
//   - Transfer-Encoding
//   - Upgrade
//   - Proxy-Connection
//   - Headers in "Connection"
func removeHopByHopHeaders(h http.Header) {
	hh := httpHeader(h) // For performance.
	// RFC 7230, section 6.1: Remove headers listed in the "Connection" header.
	for _, conn := range hh.Values("Connection") {
		for header := range strings.SplitSeq(conn, ",") {
			if header = textproto.TrimString(header); header != "" {
				h.Del(header)
			}
		}
	}
	// RFC 2616, section 13.5.1: Remove a set of known hop-by-hop headers.
	// This behavior is superseded by the RFC 7230 Connection header, but
	// preserve it for backwards compatibility.
	hh.Del("Connection")          // RFC2616 HTTP/1.1
	hh.Del("Keep-Alive")          // RFC2616 HTTP/1.1
	hh.Del("Proxy-Authenticate")  // RFC2616 HTTP/1.1
	hh.Del("Proxy-Authorization") // RFC2616 HTTP/1.1
	hh.Del("Te")                  // RFC2616 HTTP/1.1
	hh.Del("Trailer")             // RFC2616 HTTP/1.1
	hh.Del("Transfer-Encoding")   // RFC2616 HTTP/1.1
	hh.Del("Upgrade")             // RFC2616 HTTP/1.1
	hh.Del("Proxy-Connection")    // non-standard
}

// copyHeaders copies headers from src to dst.
// It does not delete existing values but append to it.
func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		k = http.CanonicalHeaderKey(k)
		dst[k] = append(dst[k], v...)
	}
}

// copyTrailers copies trailers from src to dst.
// Header keys in src are copied with the prefix defined as [http.TrailerPrefix].
// It does not delete existing trailers in dst but append to it.
func copyTrailers(dst, src http.Header) {
	for k, v := range src {
		k = http.CanonicalHeaderKey(k)
		if !strings.HasPrefix(k, http.TrailerPrefix) {
			k = http.TrailerPrefix + k
		}
		dst[k] = append(dst[k], v...)
	}
}

// upgradeType returns the string of protocol to upgrade.
// "Upgrade" header must be present at most 1 in the given h.
// The header key in the given h must be canonicalized.
// i.e. "Upgrade" not "upgrade", "Connection" not "connection".
//
// Reference
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Protocol_upgrade_mechanism
//   - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Upgrade
func upgradeType(h http.Header) string {
	if httpguts.HeaderValuesContainsToken(h.Values("Connection"), "Upgrade") {
		return h.Get("Upgrade")
	}
	return ""
}
