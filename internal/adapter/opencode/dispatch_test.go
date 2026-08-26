package opencode

import (
	"context"
	"errors"
	"kv/internal/hook"
	"kv/internal/spool"
	"testing"
	"time"
)

type failingClient struct{ fail bool }

func (c *failingClient) Handle(context.Context, hook.HookEnvelope) (hook.HookResult, error) {
	if c.fail {
		return hook.HookResult{}, errors.New("unavailable")
	}
	return hook.HookResult{Status: "processed"}, nil
}
func TestDispatcherQueuesDegradedDeliveryAndRetries(t *testing.T) {
	queue, err := spool.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client := &failingClient{fail: true}
	dispatcher := NewDispatcher(client, queue)
	envelope := hook.HookEnvelope{EventID: "event"}
	result, err := dispatcher.Deliver(context.Background(), envelope)
	if err != nil || !result.Degraded {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	due, _ := queue.Due(time.Now().Add(time.Second))
	if len(due) != 1 {
		t.Fatalf("due = %#v", due)
	}
	client.fail = false
	if err := dispatcher.RetryDue(context.Background(), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	due, _ = queue.Due(time.Now().Add(time.Second))
	if len(due) != 0 {
		t.Fatalf("due after retry = %#v", due)
	}
}
