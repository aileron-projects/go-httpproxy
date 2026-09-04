package httpproxy

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/http/httpguts"
)

// ProxyRequest represents the frontend and backend requests.
type ProxyRequest struct {
	// In is the request received by the proxy.
	// In other words, it is the request from frontend.
	// In must not be modified.
	In *http.Request
	// Out is the request which will be sent by the proxy.
	// In other words, it is the request to backend.
	// The Rewrite function may modify or replace this request.
	// Hop-by-hop headers are removed from this request
	// before Rewrite is called.
	Out *http.Request
}

// In represents the frontend request and response.
type In struct {
	// Req is the frontend-side request.
	// Req must not be modified.
	Req *http.Request
	// Res is the frontend-side response.
	// Req must not be modified.
	Res http.ResponseWriter
}

// Out represents the backend request and response.
type Out struct {
	// Req is the backend-side request.
	// Req must not be modified.
	Req *http.Request
	// Res is the backend-side response.
	// Res can be modified.
	Res *http.Response
}

// Proxy is an HTTP proxy.
// It implements [net/http.Handler].
type Proxy struct {
	// The transport used to perform proxy requests.
	// If nil, [net/http.DefaultTransport] is used.
	Transport http.RoundTripper
	// PreRoundTrip is called just before roundtrip.
	// In request must not be modified in the function.
	// Out request can be modified in the function.
	// PreRoundTrip can be used for rewriting request url
	// or changing request context and so on.
	// Proxy stops proceeding the request when PreRoundTrip
	// returns non-nil error and calls ErrorHandler.
	PreRoundTrip func(*ProxyRequest) error
	// PostRoundTrip is called just after roundtrip.
	// The given response is the response from backend.
	// In and Out requests must not be modified in PostRoundTrip.
	// In holds the frontend-side request and response.
	// Out holds the backend-side request and response.
	PostRoundTrip func(*In, *Out) error
	// ErrorHandler is the optional error handler.
	// If non-nil, any errors occurred while proxying is given.
	// If nil, a default error handler is used.
	// The ResponseWriter will be nil when response is not writable.
	ErrorHandler func(http.ResponseWriter, *http.Request, *Error)

	// TODO: Support more fine-grained connection management.
	// For example, https://github.com/golang/go/issues/75830
}

func (p *Proxy) handleError(w http.ResponseWriter, r *http.Request, err *Error) {
	// modify the err cause to [ErrClientCanceled] and mark the err non-writable
	// if the request was canceled.
	if cause := context.Cause(r.Context()); errors.Is(cause, context.Canceled) {
		err.Cause = ErrClientCanceled
		err.written = true
	}
	if !err.Writable() {
		w = nil // Response is not writable.
	}
	if eh := p.ErrorHandler; eh != nil {
		eh(w, r, err)
		return
	}
	if w != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(http.StatusText(http.StatusBadGateway)))
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	outReq := newProxyRequest(r)
	if outReq.Body != nil {
		defer outReq.Body.Close()
	}

	// Pre-round-trip hook.
	if preRT := p.PreRoundTrip; preRT != nil {
		bodyBefore := outReq.Body
		pr := &ProxyRequest{In: r, Out: outReq}
		if err := preRT(pr); err != nil {
			p.handleError(w, r, &Error{Inner: err, Cause: ErrPreRoundTrip})
			return
		}
		outReq = pr.Out // Request may be changed.
		bodyAfter := outReq.Body
		if bodyAfter != nil && bodyAfter != bodyBefore { // Check if the request body replaced.
			defer bodyAfter.Close() // Close replaced request body.
		}
	}

	outCtx, cancel := context.WithCancel(outReq.Context())
	defer cancel()
	outReq = outReq.WithContext(outCtx)

	transport := cmp.Or(p.Transport, http.DefaultTransport)
	outRes, err := transport.RoundTrip(outReq)
	if err != nil {
		p.handleError(w, r, &Error{Inner: err, Cause: ErrRoundTrip})
		return
	}
	defer outRes.Body.Close()

	// Post-round-trip hook.
	if postRT := p.PostRoundTrip; postRT != nil {
		in := &In{Req: r, Res: w}
		out := &Out{Req: outReq, Res: outRes}
		bodyBefore := outRes.Body
		if err := postRT(in, out); err != nil {
			p.handleError(w, r, &Error{Inner: err, Cause: ErrPostRoundTrip})
			return
		}
		outRes = out.Res // Response may be changed.
		bodyAfter := outRes.Body
		if bodyAfter != nil && bodyAfter != bodyBefore { // Check if the request body replaced.
			defer bodyAfter.Close() // Close replaced response body.
		}
	}

	// Deal with 101 Switching Protocols responses: (WebSocket, h2c, etc)
	if outRes.StatusCode == http.StatusSwitchingProtocols {
		if err := handleUpgradeResponse(w, outReq, outRes); err != nil {
			p.handleError(w, r, err)
		}
		return
	}

	if err := sendResponse(w, outRes); err != nil {
		p.handleError(w, r, err)
	}
}

// newProxyRequest returns a new request that should be sent by proxy to backend.
func newProxyRequest(in *http.Request) *http.Request {
	out := in.Clone(in.Context())
	out.Host = ""
	if in.ContentLength == 0 {
		out.Body = nil // Issue 16036: nil Body for http.Transport retries
	}
	if out.Header == nil {
		out.Header = make(http.Header, 0)
	}
	if _, ok := out.Header["User-Agent"]; !ok {
		out.Header.Set("User-Agent", "") // Don't send the default Go User-Agent.
	}
	removeHopByHopHeaders(out.Header)
	if httpguts.HeaderValuesContainsToken(in.Header["Te"], "trailers") {
		out.Header.Set("Te", "trailers")
	}
	if httpguts.HeaderValuesContainsToken(in.Header["Connection"], "upgrade") {
		out.Header.Set("Connection", "upgrade")
		out.Header.Set("Upgrade", in.Header.Get("Upgrade"))
	}
	return out
}

// sendResponse sends the backend response to the frontend.
// in is the frontend response writer.
// out is the backend response.
func sendResponse(in http.ResponseWriter, out *http.Response) *Error {
	outHeader := out.Header.Clone() // Do not modify out.Header directly.
	removeHopByHopHeaders(outHeader)
	copyHeaders(in.Header(), outHeader)
	if n := len(out.Trailer); n > 0 {
		announcedTrailerKeys := make([]string, 0, n)
		for k := range out.Trailer {
			announcedTrailerKeys = append(announcedTrailerKeys, k)
		}
		in.Header().Add("Trailer", strings.Join(announcedTrailerKeys, ", "))
	}
	in.WriteHeader(out.StatusCode)
	if err := copyResponseBody(in, out); err != nil {
		return &Error{Inner: err, Cause: ErrWriteResponse, written: true}
	}

	_ = out.Body.Close() // Close before populating trailers.

	if len(out.Trailer) > 0 {
		// Force chunking if we saw a response trailer.
		if err := http.NewResponseController(in).Flush(); err != nil {
			return &Error{Inner: err, Cause: ErrFlushResponse, written: true}
		}
		copyTrailers(in.Header(), out.Trailer)
	}
	return nil
}

// handleUpgradeResponse handles protocol upgrade.
// This method is called when [net/http.StatusSwitchingProtocols] was detected.
// See also handleUpgradeResponse function in
// https://go.dev/src/net/http/httputil/reverseproxy.go
func handleUpgradeResponse(inRes http.ResponseWriter, outReq *http.Request, outRes *http.Response) *Error {
	requestedType := upgradeType(outReq.Header)
	respondedType := upgradeType(outRes.Header)
	if !strings.EqualFold(requestedType, respondedType) {
		return &Error{
			Cause:  ErrProtocolMismatch,
			Detail: fmt.Sprintf("backend tried to switch protocol %q when %q was requested", respondedType, requestedType),
		}
	}

	backConn, ok := outRes.Body.(io.ReadWriteCloser)
	if !ok {
		return &Error{
			Cause:  ErrNonSwitchable,
			Detail: "101 switching protocols response with non-writable body (" + fmt.Sprintf("%T", outRes.Body) + ")",
		}
	}
	defer backConn.Close() // Ensure close.

	// Ensure that the cancellation of a request closes the backend.
	// See issue https://golang.org/issue/35559.
	backConnCloseCh := make(chan bool)
	defer close(backConnCloseCh)
	go func() {
		select {
		case <-outReq.Context().Done():
		case <-backConnCloseCh:
		}
		backConn.Close()
	}()

	frontConn, brw, err := http.NewResponseController(inRes).Hijack()
	if err != nil {
		return &Error{
			Inner:  err,
			Cause:  ErrNonSwitchable,
			Detail: "can't switch protocols using non-Hijacker ResponseWriter (" + fmt.Sprintf("%T", inRes) + ")",
		}
	}
	defer frontConn.Close() // Ensure close.

	copyHeaders(inRes.Header(), outRes.Header)

	resp := outRes               // Shallow copy response to generate a response.
	resp.Header = inRes.Header() // Headers to be responded.
	resp.Body = nil              // Avoid writing body. We copy using frontConn and backConn.
	if err := resp.Write(brw); err != nil {
		return &Error{Inner: err, Cause: ErrWriteResponse, written: true}
	}
	if err := brw.Flush(); err != nil {
		return &Error{Inner: err, Cause: ErrFlushResponse, written: true}
	}

	errChan := make(chan error, 2)
	go copyBuf(frontConn, backConn, errChan)
	go copyBuf(backConn, frontConn, errChan)
	err1 := <-errChan // Wait for one copyBuf call to finish.
	err2 := <-errChan // Wait for the other copyBuf call to finish.
	if err := cmp.Or(err1, err2); err != nil {
		return &Error{Inner: err, Cause: ErrBidirectionalCom, written: true}
	}
	return nil
}
