package httpproxy

import (
	"net/url"
	"sync"
)

// NewProxy returns a new instance of [Proxy] with the given
// proxy targets. Targets are selected by round-robin algorithm.
func NewProxy(targets ...string) (*Proxy, error) {
	ts := make([]*url.URL, 0, len(targets))
	for _, t := range targets {
		u, err := url.Parse(t)
		if err != nil {
			return nil, err
		}
		ts = append(ts, u)
	}
	var mu sync.Mutex
	var index int
	return &Proxy{
		PreRoundTrip: func(pr *ProxyRequest) error {
			mu.Lock()
			defer mu.Unlock()
			if index >= len(ts) {
				index = 0
			}
			target := ts[index]
			index++
			RewriteURL(pr.Out.URL, target)
			SetViaHeader(pr.In, pr.Out.Header, "", "")
			SetForwardedHeader(pr.In, pr.Out.Header)
			SetXForwardedHeaders(pr.In, pr.Out.Header)
			return nil
		},
	}, nil
}

// RewriteURL rewrites target url with templated url.
func RewriteURL(target, tpl *url.URL) {
	target.Scheme = tpl.Scheme
	target.User = tpl.User
	target.Host = tpl.Host
	target.Path = joinWithByte(tpl.Path, target.Path, '/')
	target.RawPath = joinWithByte(tpl.RawPath, target.RawPath, '/')
	target.Fragment = tpl.Fragment
	target.RawFragment = tpl.RawFragment
	target.RawQuery = joinWithByte(tpl.RawQuery, target.RawQuery, '&')
	target.ForceQuery = tpl.ForceQuery
	target.OmitHost = tpl.OmitHost
}

func joinWithByte(left, right string, b byte) string {
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	leftHasByte := left[len(left)-1] == b
	rightHasByte := right[0] == b
	if leftHasByte && rightHasByte {
		return left + right[1:]
	}
	if !leftHasByte && !rightHasByte {
		return left + string(b) + right
	}
	return left + right
}
