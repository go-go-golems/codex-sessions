package sessions

import "fmt"

// ParseError annotates a JSON parsing error with file and line number context.
type ParseError struct {
	Path   string
	LineNo int
	Err    error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d: %v", e.Path, e.LineNo, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }
