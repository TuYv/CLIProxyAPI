package usage

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestManagerPublishClonesResponseHeaders(t *testing.T) {
	gate := make(chan struct{})
	records := make(chan Record, 1)
	manager := NewManager(1)
	manager.Register(gatedCapturePlugin{gate: gate, records: records})
	defer manager.Stop()

	headers := http.Header{}
	headers.Set("X-Upstream-Request-Id", "initial")

	manager.Publish(context.Background(), Record{ResponseHeaders: headers})
	headers.Set("X-Upstream-Request-Id", "mutated")
	close(gate)

	select {
	case record := <-records:
		if got := record.ResponseHeaders.Get("X-Upstream-Request-Id"); got != "initial" {
			t.Fatalf("ResponseHeaders = %q, want %q", got, "initial")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for usage record")
	}
}

type gatedCapturePlugin struct {
	gate    <-chan struct{}
	records chan<- Record
}

func (p gatedCapturePlugin) HandleUsage(_ context.Context, record Record) {
	<-p.gate
	p.records <- record
}
