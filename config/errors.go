package config

import "fmt"

// ParseError describes a malformed TLC configuration and its Java diagnostic.
type ParseError struct {
	Message   string
	Code      int
	Line      int
	Parameter string
	Exception string
}

func (e *ParseError) Error() string { return e.Message }

func nullTokenError() error {
	return &ParseError{Message: `Cannot invoke "String.equals(Object)" because "tt.image" is null`, Exception: "java.lang.NullPointerException"}
}

// Diagnostic returns the message used inside TLC's configuration exception.
func (e *ParseError) Diagnostic() string {
	prefix := fmt.Sprintf("TLC found an error in the configuration file at line %d\n", e.Line)
	switch e.Code {
	case 5002:
		return prefix + "\n"
	case 5003:
		return prefix + fmt.Sprintf("The keyword %s was not followed by an identifier.", e.Parameter)
	case 5004:
		return prefix + fmt.Sprintf("The keyword %s appeared twice.", e.Parameter)
	case 5005:
		return prefix + fmt.Sprintf("Expected an identifier after %s.", e.Parameter)
	case 5006:
		return prefix + fmt.Sprintf("It was expecting %s, but did not find it.", e.Parameter)
	default:
		return e.Message
	}
}
