package workers

import (
	"context"

	"github.com/Codi-Devs/ventago-homelab/internal/config"
)

const (
	NRMonitor     = "nr_monitor"
	MorningDigest = "morning_digest"
	OCR           = "ocr"
)

// Worker is a homelab job the orchestrator can dispatch.
type Worker interface {
	Name() string
	Run(ctx context.Context) error
}

type liveWorker interface {
	Live() bool
}

type Stub struct {
	Kind string
}

func (s Stub) Name() string { return s.Kind }

func (s Stub) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func Catalog(cfg config.Config) []Worker {
	return []Worker{
		Stub{Kind: NRMonitor},
		Stub{Kind: MorningDigest},
		NewOCRWorker(cfg),
	}
}

func StatusLabel(wrk Worker, enabled bool) string {
	if !enabled {
		if _, ok := wrk.(liveWorker); ok {
			return "disabled"
		}
		return "stub_disabled"
	}
	if live, ok := wrk.(liveWorker); ok && live.Live() {
		return "enabled"
	}
	return "stub_enabled"
}
