package errors

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrAccessDenied   = errors.New("access denied")
	ErrAlreadyExists  = errors.New("already exists")
	ErrWidgetDisabled = errors.New("widget is disabled")
)
