package httpproxy

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/aileron-projects/go-tester"
)

func TestError(t *testing.T) {
	t.Parallel()
	t.Run("unwrap", func(t *testing.T) {
		err := &Error{Inner: io.EOF}
		inner := err.Unwrap()
		tester.AssertEqualErr(t, io.EOF, inner)
	})
	t.Run("error message", func(t *testing.T) {
		err := &Error{Cause: io.EOF}
		msg := err.Error()
		tester.AssertEqual(t, "go-httpproxy/httpproxy: EOF", msg)
	})
	t.Run("error message with detail", func(t *testing.T) {
		err := &Error{Cause: io.EOF, Detail: "detail"}
		msg := err.Error()
		tester.AssertEqual(t, "go-httpproxy/httpproxy: EOF. detail", msg)
	})
	t.Run("error message with detail and inner", func(t *testing.T) {
		err := &Error{Inner: io.ErrUnexpectedEOF, Cause: io.EOF, Detail: "detail"}
		msg := err.Error()
		tester.AssertEqual(t, "go-httpproxy/httpproxy: EOF. detail [unexpected EOF]", msg)
	})
	t.Run("nil error", func(t *testing.T) {
		var err *Error
		tester.AssertEqual(t, false, err.Is(nil))
	})
	t.Run("nil target", func(t *testing.T) {
		err := &Error{Cause: io.EOF}
		tester.AssertEqual(t, false, err.Is(nil))
	})
	t.Run("errors equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		err := &Error{Cause: io.EOF}
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("errors not equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		err := &Error{Cause: io.ErrUnexpectedEOF}
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
	t.Run("wrapped error equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		inner := &Error{Cause: io.EOF}
		err := fmt.Errorf("outer error [%w]", inner)
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("wrapped error not equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		err := fmt.Errorf("outer error [%w]", io.ErrUnexpectedEOF)
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
	t.Run("wrapped errors equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		inner := &Error{Cause: io.EOF}
		err := fmt.Errorf("outer error [%w] [%w]", io.EOF, inner)
		tester.AssertEqual(t, true, errors.Is(err, target))
	})
	t.Run("wrapped error not equal", func(t *testing.T) {
		target := &Error{Cause: io.EOF}
		err := fmt.Errorf("outer error [%w] [%w]", io.EOF, io.ErrUnexpectedEOF)
		tester.AssertEqual(t, false, errors.Is(err, target))
	})
}
