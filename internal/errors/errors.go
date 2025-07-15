package errors

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrAccessDenied = errors.New("access denied")
	ErrAlreadyExists = errors.New("already exists")
	ErrFormDisabled = errors.New("form is disabled")
)
