package httpproxy

import (
	"errors"
)

var (
	ErrPreRoundTrip     = errors.New("pre round-trip hook failed")
	ErrPostRoundTrip    = errors.New("post round-trip hook failed")
	ErrClientCanceled   = errors.New("client canceled request")
	ErrRoundTrip        = errors.New("round-trip failed")
	ErrWriteResponse    = errors.New("writing response to client failed")
	ErrFlushResponse    = errors.New("flushing response to client failed")
	ErrProtocolMismatch = errors.New("protocol switch: protocols mismatch")
	ErrNonSwitchable    = errors.New("protocol switch: unable to switch protocol")
	ErrBidirectionalCom = errors.New("protocol switch: bidirectional communication failed")
)

// Error is the proxy error.
type Error struct {
	Inner  error  // Inner is the inner error.
	Cause  error  // Cause is the error cause in the proxy.
	Detail string // Detail is the error detail.
	// written indicates that the response has already written.
	// Any error response must not be written to response writer.
	written bool
}

// Writable reports if an error response is writable to the ResponseWriter.
// Do not write status code or bodies to ResponseWriter in error handler when false.
func (e *Error) Writable() bool {
	return !e.written
}

func (e *Error) Unwrap() error {
	return e.Inner
}

func (e *Error) Error() string {
	s := "go-httpproxy/httpproxy: " + e.Cause.Error()
	if e.Detail != "" {
		s += ". " + e.Detail
	}
	if e.Inner != nil {
		s = s + " [" + e.Inner.Error() + "]"
	}
	return s
}

func (e *Error) Is(target error) bool {
	ee, ok := target.(*Error)
	if ok {
		return e.Cause == ee.Cause
	}
	return false
}
