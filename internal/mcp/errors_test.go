package mcp

import (
	"errors"
	"testing"
)

func TestResponseForErrorIsStructuredAndDoesNotExposeInternals(t *testing.T) {
	retryable := responseForError(&ServiceError{Kind: ErrorRetryable, Message: "KV assistance is temporarily unavailable", Retry: true})
	if retryable.Code != -32001 || retryable.Data == nil {
		t.Fatalf("retryable = %#v", retryable)
	}
	validation := responseForError(errors.New("sqlite: locked"))
	if validation.Message != "invalid request" || validation.Data == nil {
		t.Fatalf("validation = %#v", validation)
	}
}
