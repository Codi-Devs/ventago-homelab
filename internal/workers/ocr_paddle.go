package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type paddleResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

func (w *OCRWorker) runPaddle(ctx context.Context, job *homelabJob) (string, float64, error) {
	localPath, err := w.downloadFile(ctx, job)
	if err != nil {
		return "", 0, err
	}
	script := w.cfg.OCRWorkDir + "/paddle_extract.py"
	args := []string{"exec", w.cfg.PaddleContainer, "python3", script, localPath}
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("paddleocr: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	var out paddleResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); err != nil {
		return "", 0, fmt.Errorf("paddleocr json: %w", err)
	}
	if strings.TrimSpace(out.Text) == "" {
		return "", 0, fmt.Errorf("paddleocr returned empty text")
	}
	if out.Confidence == 0 {
		out.Confidence = 0.9
	}
	return strings.TrimSpace(out.Text), out.Confidence, nil
}
