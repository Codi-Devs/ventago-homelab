package workers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultMetricsLog = "/var/log/ventago/ocr-metrics.log"

func metricsLogPath() string {
	if p := strings.TrimSpace(os.Getenv("OCR_METRICS_LOG")); p != "" {
		return p
	}
	return defaultMetricsLog
}

func appendMetrics(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	log.Print(line)
	path := metricsLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s\n", line)
}

func formatHw(s hwSnapshot) string {
	pkg := "na"
	if s.OKPackage {
		pkg = fmt.Sprintf("%.1f", s.PackageC)
	}
	fan := "na"
	if s.OKFan {
		fan = strconvI(s.FanRPM)
	}
	return fmt.Sprintf("package_c=%s fan_rpm=%s pwm=%d pwm_pct=%d", pkg, fan, s.PWM, s.PWMPct)
}

func strconvI(n int) string { return fmt.Sprintf("%d", n) }

type llmStats struct {
	EvalCount            int   `json:"eval_count"`
	EvalDurationNS       int64 `json:"eval_duration"`
	PromptEvalCount      int   `json:"prompt_eval_count"`
	PromptEvalDurationNS int64 `json:"prompt_eval_duration"`
	TotalDurationNS      int64 `json:"total_duration"`
	LoadDurationNS       int64 `json:"load_duration"`
}

func (s llmStats) genTokPerSec() float64 {
	if s.EvalDurationNS <= 0 || s.EvalCount <= 0 {
		return 0
	}
	return float64(s.EvalCount) / (float64(s.EvalDurationNS) / 1e9)
}

func (s llmStats) promptTokPerSec() float64 {
	if s.PromptEvalDurationNS <= 0 || s.PromptEvalCount <= 0 {
		return 0
	}
	return float64(s.PromptEvalCount) / (float64(s.PromptEvalDurationNS) / 1e9)
}

func (s llmStats) logFields() string {
	return fmt.Sprintf(
		"prompt_tokens=%d gen_tokens=%d prompt_tok_s=%.2f gen_tok_s=%.2f load_ms=%.0f prompt_eval_ms=%.0f gen_ms=%.0f total_ms=%.0f",
		s.PromptEvalCount,
		s.EvalCount,
		s.promptTokPerSec(),
		s.genTokPerSec(),
		float64(s.LoadDurationNS)/1e6,
		float64(s.PromptEvalDurationNS)/1e6,
		float64(s.EvalDurationNS)/1e6,
		float64(s.TotalDurationNS)/1e6,
	)
}

func watchPhase(jobID int64, phase string) func(extra string) {
	start := readHwSnapshot()
	appendMetrics(fmt.Sprintf("%s job=%d phase=%s event=start %s", start.At.Format(time.RFC3339), jobID, phase, formatHw(start)))

	var mu sync.Mutex
	maxPkg := start.PackageC
	maxFan := start.FanRPM
	samples := 1

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				s := readHwSnapshot()
				mu.Lock()
				samples++
				if s.OKPackage && s.PackageC > maxPkg {
					maxPkg = s.PackageC
				}
				if s.OKFan && s.FanRPM > maxFan {
					maxFan = s.FanRPM
				}
				mu.Unlock()
				appendMetrics(fmt.Sprintf("%s job=%d phase=%s event=sample %s", s.At.Format(time.RFC3339), jobID, phase, formatHw(s)))
			}
		}
	}()

	began := time.Now()
	return func(extra string) {
		close(done)
		wg.Wait()
		end := readHwSnapshot()
		mu.Lock()
		if end.OKPackage && end.PackageC > maxPkg {
			maxPkg = end.PackageC
		}
		if end.OKFan && end.FanRPM > maxFan {
			maxFan = end.FanRPM
		}
		n := samples
		mu.Unlock()
		line := fmt.Sprintf(
			"%s job=%d phase=%s event=done duration_s=%.1f samples=%d package_c_start=%.1f package_c_max=%.1f package_c_end=%.1f fan_rpm_start=%d fan_rpm_max=%d fan_rpm_end=%d pwm_end=%d %s",
			end.At.Format(time.RFC3339),
			jobID,
			phase,
			time.Since(began).Seconds(),
			n,
			start.PackageC,
			maxPkg,
			end.PackageC,
			start.FanRPM,
			maxFan,
			end.FanRPM,
			end.PWM,
			formatHw(end),
		)
		if extra = strings.TrimSpace(extra); extra != "" {
			line += " " + extra
		}
		appendMetrics(line)
	}
}
