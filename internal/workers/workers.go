package workers

import "context"

const (
	NRMonitor     = "nr_monitor"
	MorningDigest = "morning_digest"
	OCR           = "ocr"
)

// Worker is a homelab job the orchestrator can dispatch. Implementations for
// New Relic monitor, morning digest and OCR are intentionally stubs until those
// features are built.
type Worker interface {
	Name() string
	Run(ctx context.Context) error
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

func Catalog() []Worker {
	return []Worker{
		Stub{Kind: NRMonitor},
		Stub{Kind: MorningDigest},
		Stub{Kind: OCR},
	}
}
