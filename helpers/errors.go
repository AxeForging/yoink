package helpers

import "fmt"

// YoinkError is a structured error with operation context.
type YoinkError struct {
	Operation string
	Details   string
	Err       error
}

func (e *YoinkError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Operation, e.Details, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Details)
}

func (e *YoinkError) Unwrap() error {
	return e.Err
}

// WrapError wraps an error with operation context. Returns nil if err is nil.
func WrapError(err error, operation, details string) error {
	if err == nil {
		return nil
	}
	return &YoinkError{Operation: operation, Details: details, Err: err}
}
