package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Codi-Devs/ventago-homelab/internal/config"
)

type OCRWorker struct {
	cfg    config.Config
	apiKey string
	http   *http.Client
	store  *ocrStore
}

func NewOCRWorker(cfg config.Config) *OCRWorker {
	return &OCRWorker{
		cfg:    cfg,
		apiKey: strings.TrimSpace(os.Getenv("HOMELAB_OCR_API_KEY")),
		http:   &http.Client{Timeout: 3 * time.Minute},
	}
}

func (w *OCRWorker) Name() string { return OCR }
func (w *OCRWorker) Live() bool   { return true }

func (w *OCRWorker) Run(ctx context.Context) error {
	if strings.TrimSpace(w.apiKey) == "" || strings.TrimSpace(w.cfg.VentagoAPIBase) == "" {
		return nil
	}
	if hot, err := thermalTooHot(); err != nil {
		log.Printf("ocr thermal: %v", err)
	} else if hot {
		return fmt.Errorf("thermal pause: package >= 85C")
	}
	if err := w.ensureStore(); err != nil {
		log.Printf("ocr mysql: %v", err)
	}
	return w.processOne(ctx)
}

func (w *OCRWorker) ensureStore() error {
	if w.store != nil || strings.TrimSpace(w.cfg.MySQLDSN) == "" {
		return nil
	}
	store, err := openOCRStore(w.cfg.MySQLDSN)
	if err != nil {
		return err
	}
	w.store = store
	return nil
}

func (w *OCRWorker) processOne(ctx context.Context) error {
	job, err := w.lease(ctx)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}
	if w.store != nil {
		_ = w.store.MarkSeen(job.ID, job.Route)
	}

	text := strings.TrimSpace(job.ExtractedText)
	if strings.EqualFold(job.Route, "ocr") && text == "" {
		ocrText, conf, ocrErr := w.runPaddle(ctx, job)
		if ocrErr != nil {
			_ = w.fail(ctx, job.ID, ocrErr.Error())
			return ocrErr
		}
		updated, resErr := w.postOCRResult(ctx, job.ID, ocrText, conf)
		if resErr != nil {
			_ = w.fail(ctx, job.ID, resErr.Error())
			return resErr
		}
		text = strings.TrimSpace(updated.ExtractedText)
		if text == "" {
			text = ocrText
		}
	}
	if text == "" {
		err := fmt.Errorf("empty invoice text")
		_ = w.fail(ctx, job.ID, err.Error())
		return err
	}

	parsed, parseErr := w.parseWithLLM(ctx, text, "")
	if parseErr != nil {
		parsed, parseErr = w.parseWithLLM(ctx, text, parseErr.Error())
	}
	if parseErr != nil {
		_ = w.fail(ctx, job.ID, parseErr.Error())
		if w.store != nil {
			_ = w.store.MarkError(job.ID, parseErr.Error())
		}
		return parseErr
	}

	if err := w.complete(ctx, job.ID, parsed); err != nil {
		_ = w.fail(ctx, job.ID, err.Error())
		return err
	}
	if w.store != nil {
		_ = w.store.MarkDone(job.ID)
	}
	return nil
}

type leaseEnvelope struct {
	Job *homelabJob `json:"job"`
}

type homelabJob struct {
	ID            int64  `json:"id"`
	BusinessID    int    `json:"business_id"`
	Route         string `json:"route"`
	Status        string `json:"status"`
	FileURL       string `json:"file_url"`
	FileMime      string `json:"file_mime"`
	FileName      string `json:"file_name"`
	ExtractedText string `json:"extracted_text"`
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *string         `json:"error"`
}

func (w *OCRWorker) lease(ctx context.Context) (*homelabJob, error) {
	var env apiEnvelope
	if err := w.doJSON(ctx, http.MethodGet, "/api/v1/internal/homelab/ocr/lease", nil, &env); err != nil {
		return nil, err
	}
	if !env.Success {
		return nil, fmt.Errorf("lease failed")
	}
	var payload leaseEnvelope
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, err
	}
	return payload.Job, nil
}

func (w *OCRWorker) postOCRResult(ctx context.Context, id int64, text string, confidence float64) (*homelabJob, error) {
	body, _ := json.Marshal(map[string]any{"text": text, "confidence": confidence})
	var env apiEnvelope
	if err := w.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/internal/homelab/ocr/%d/ocr-result", id), body, &env); err != nil {
		return nil, err
	}
	if !env.Success {
		return nil, fmt.Errorf("ocr-result failed")
	}
	var payload leaseEnvelope
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		return nil, err
	}
	if payload.Job == nil {
		return &homelabJob{ID: id, ExtractedText: text, Route: "ocr"}, nil
	}
	return payload.Job, nil
}

func (w *OCRWorker) complete(ctx context.Context, id int64, raw json.RawMessage) error {
	var env apiEnvelope
	if err := w.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/internal/homelab/ocr/%d/complete", id), raw, &env); err != nil {
		return err
	}
	if !env.Success {
		return fmt.Errorf("complete failed")
	}
	return nil
}

func (w *OCRWorker) fail(ctx context.Context, id int64, message string) error {
	body, _ := json.Marshal(map[string]string{"error": message})
	var env apiEnvelope
	return w.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/v1/internal/homelab/ocr/%d/fail", id), body, &env)
}

func (w *OCRWorker) doJSON(ctx context.Context, method, path string, body []byte, dest *apiEnvelope) error {
	url := strings.TrimRight(w.cfg.VentagoAPIBase, "/") + path
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-Homelab-Key", w.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func (w *OCRWorker) downloadFile(ctx context.Context, job *homelabJob) (string, error) {
	if err := os.MkdirAll(w.cfg.OCRWorkDir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(job.FileName)
	if ext == "" {
		if strings.Contains(strings.ToLower(job.FileMime), "png") {
			ext = ".png"
		} else if strings.Contains(strings.ToLower(job.FileMime), "pdf") {
			ext = ".pdf"
		} else {
			ext = ".jpg"
		}
	}
	dest := filepath.Join(w.cfg.OCRWorkDir, fmt.Sprintf("job-%d%s", job.ID, ext))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, job.FileURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", err
	}
	return dest, nil
}
