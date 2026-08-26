package mcp

import "errors"

type ErrorKind string

const (
	ErrorDegraded           ErrorKind = "degraded"
	ErrorValidation         ErrorKind = "validation"
	ErrorRetryable          ErrorKind = "retryable"
	ErrorConflict           ErrorKind = "conflict"
	ErrorUnsupportedVersion ErrorKind = "unsupported_version"
)

// ServiceError is the storage-independent error contract exposed to MCP
// clients. Detail is safe operational guidance, never a database error.
type ServiceError struct {
	Kind    ErrorKind
	Message string
	Retry   bool
}

func (e *ServiceError) Error() string { return e.Message }

func responseForError(err error) *ResponseError {
	var serviceError *ServiceError
	if errors.As(err, &serviceError) {
		code := -32602
		if serviceError.Kind == ErrorRetryable || serviceError.Kind == ErrorDegraded {
			code = -32001
		}
		if serviceError.Kind == ErrorConflict {
			code = -32009
		}
		return &ResponseError{Code: code, Message: serviceError.Message, Data: map[string]any{"kind": serviceError.Kind, "retryable": serviceError.Retry}}
	}
	return &ResponseError{Code: -32602, Message: "invalid request", Data: map[string]any{"kind": ErrorValidation, "retryable": false}}
}
