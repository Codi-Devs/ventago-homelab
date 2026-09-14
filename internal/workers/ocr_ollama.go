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
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Format   json.RawMessage `json:"format"`
	Messages []ollamaMessage `json:"messages"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}

func (w *OCRWorker) parseWithLLM(ctx context.Context, invoiceText, previousError string) (json.RawMessage, error) {
	sys := `Eres un extractor de facturas de Panama. Responde SOLO JSON valido que cumpla el schema. Moneda USD. No inventes CUFE. No escribas prosa.`
	user := "Texto de la factura:\n" + invoiceText
	if previousError != "" {
		user += "\n\nEl JSON anterior fallo validacion: " + previousError + "\nCorrige y vuelve a emitir JSON."
	}
	body, err := json.Marshal(ollamaChatRequest{
		Model:  w.cfg.OllamaModel,
		Stream: false,
		Format: invoiceJSONSchema,
		Messages: []ollamaMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(w.cfg.OllamaHost, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama HTTP %d", resp.StatusCode)
	}
	var chat ollamaChatResponse
	if err := json.Unmarshal(raw, &chat); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(chat.Message.Content)
	if !json.Valid([]byte(content)) {
		return nil, fmt.Errorf("llm did not return json")
	}
	if err := validateInvoiceJSON([]byte(content)); err != nil {
		return nil, err
	}
	return json.RawMessage(content), nil
}
