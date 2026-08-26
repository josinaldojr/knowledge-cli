package codex

import (
	"context"
	"fmt"
	"time"

	"kv/internal/hook"
	"kv/internal/spool"
)

type HookClient interface {
	Handle(context.Context, hook.HookEnvelope) (hook.HookResult, error)
}

type Dispatcher struct {
	client HookClient
	spool  *spool.Spool
}

func NewDispatcher(client HookClient, spool *spool.Spool) *Dispatcher {
	return &Dispatcher{client: client, spool: spool}
}

func (d *Dispatcher) Deliver(ctx context.Context, envelope hook.HookEnvelope) (hook.HookResult, error) {
	result, err := d.client.Handle(ctx, envelope)
	if err == nil {
		return result, nil
	}
	if d.spool == nil {
		return hook.HookResult{Status: "degraded", Degraded: true, Diagnostics: []hook.Diagnostic{{Code: "kv_delivery_degraded", Severity: "warning", Message: "KV assistance is unavailable; Codex work may continue"}}}, nil
	}
	if err := d.spool.Enqueue(spool.Entry{Envelope: envelope, RetryAt: time.Now().UTC()}); err != nil {
		return hook.HookResult{}, fmt.Errorf("queue degraded Codex delivery: %w", err)
	}
	return hook.HookResult{Status: "degraded", Degraded: true, Diagnostics: []hook.Diagnostic{{Code: "kv_delivery_queued", Severity: "warning", Message: "KV assistance is degraded; lifecycle event queued for retry"}}}, nil
}

func (d *Dispatcher) RetryDue(ctx context.Context, now time.Time) error {
	if d.spool == nil {
		return nil
	}
	entries, err := d.spool.Due(now)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := d.client.Handle(ctx, entry.Envelope); err != nil {
			if err := d.spool.Retry(entry, now); err != nil {
				return err
			}
			continue
		}
		if err := d.spool.Acknowledge(entry.Envelope.EventID); err != nil {
			return err
		}
	}
	return nil
}
