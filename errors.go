package bibtex

import (
	"errors"
	"fmt"
)

var (
	ErrLimit       = errors.New("resource limit exceeded")
	ErrOptions     = errors.New("invalid parse options")
	ErrUnsupported = errors.New("unsupported input")
)

// Position identifies a one-based source line and column. Columns count runes.
// Zero means unavailable.
type Position struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

// Diagnostic describes a recoverable problem without requiring consumers to
// parse its message.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Position
}

// Error wraps a failure that stopped parsing, for errors.Is and errors.As.
type Error struct {
	Diagnostic
	Cause error
}

func (e *Error) Error() string {
	if e.Line != 0 {
		return fmt.Sprintf("bibtex: %d:%d: %s", e.Line, e.Column, e.Message)
	}
	return "bibtex: " + e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func failure(cause error, code, message string, pos Position) error {
	return &Error{Diagnostic: Diagnostic{Code: code, Message: message, Position: pos}, Cause: cause}
}
