package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type ollamaChatRequest struct {
	Model     string          `json:"model"`
	Stream    bool            `json:"stream"`
	Think     bool            `json:"think"`
	KeepAlive string          `json:"keep_alive,omitempty"`
	Format    json.RawMessage `json:"format"`
	Messages  []ollamaMessage `json:"messages"`
	Options   ollamaOptions   `json:"options"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message            ollamaMessage `json:"message"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       int64         `json:"eval_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration int64         `json:"prompt_eval_duration"`
	TotalDuration      int64         `json:"total_duration"`
	LoadDuration       int64         `json:"load_duration"`
}

func (w *OCRWorker) parseWithLLM(ctx context.Context, invoiceText, previousError string) (json.RawMessage, llmStats, error) {
	sys := `You extract Panama invoices. Reply with schema JSON only. Currency USD. Do not invent CUFE. No prose.
total_amount is the printed TOTAL (e.g. B/. 40.10). Never add ITBMS on top of TOTAL: SUBTTL + ITBMS already equals TOTAL.
subtotal is SUBTTL. itbms_total is the ITBMS line. Line amounts (B/. 19.99) go in item.subtotal and item.unit_price (if qty=1, unit_price equals that amount; never 0). item.total includes that line's ITBMS; sum(item.total) must equal the printed TOTAL, not SUBTTL and not TOTAL+ITBMS.
issuer.dv and receiver.dv are the Panama RUC check digit only: digits, 1 or 2 characters. Valid examples: "69", "02", "2". Never country, city, letters, or VAT IDs. Use "" for foreign companies (Ireland, US, etc.) that have no Panama DV.`
	user := "Invoice text:\n" + invoiceText
	if previousError != "" {
		user += "\n\nPrevious JSON failed validation: " + previousError + "\nFix and emit JSON again."
	}
	var stats llmStats
	body, err := json.Marshal(ollamaChatRequest{
		Model:     w.cfg.OllamaModel,
		Stream:    false,
		Think:     false,
		KeepAlive: "30m",
		Format:    invoiceJSONSchema,
		Messages: []ollamaMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		Options: ollamaOptions{Temperature: 0},
	})
	if err != nil {
		return nil, stats, err
	}
	url := strings.TrimRight(w.cfg.OllamaHost, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, stats, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, stats, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, stats, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, stats, fmt.Errorf("ollama HTTP %d", resp.StatusCode)
	}
	var chat ollamaChatResponse
	if err := json.Unmarshal(raw, &chat); err != nil {
		return nil, stats, err
	}
	stats = llmStats{
		EvalCount:            chat.EvalCount,
		EvalDurationNS:       chat.EvalDuration,
		PromptEvalCount:      chat.PromptEvalCount,
		PromptEvalDurationNS: chat.PromptEvalDuration,
		TotalDurationNS:      chat.TotalDuration,
		LoadDurationNS:       chat.LoadDuration,
	}
	content := strings.TrimSpace(chat.Message.Content)
	if !json.Valid([]byte(content)) {
		return nil, stats, fmt.Errorf("llm did not return json")
	}
	normalized, err := normalizeInvoiceJSON([]byte(content))
	if err != nil {
		return nil, stats, err
	}
	return json.RawMessage(normalized), stats, nil
}
