package workers

import "testing"

func TestLLMStatsTokPerSec(t *testing.T) {
	s := llmStats{EvalCount: 100, EvalDurationNS: 2e9, PromptEvalCount: 50, PromptEvalDurationNS: 1e9}
	if got := s.genTokPerSec(); got < 49.9 || got > 50.1 {
		t.Fatalf("genTokPerSec=%v want 50", got)
	}
	if got := s.promptTokPerSec(); got < 49.9 || got > 50.1 {
		t.Fatalf("promptTokPerSec=%v want 50", got)
	}
	if s.logFields() == "" {
		t.Fatal("empty log fields")
	}
}

func TestErrString(t *testing.T) {
	if errString(nil) != "" {
		t.Fatal("nil")
	}
}
