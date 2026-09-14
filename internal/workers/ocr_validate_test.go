package workers

import (
	"testing"

	"github.com/Codi-Devs/ventago-homelab/internal/config"
)

func TestValidateInvoiceJSON(t *testing.T) {
	ok := []byte(`{
		"invoice_number":"A-1",
		"emission_date":"2026-09-01",
		"issuer":{"name":"Proveedor"},
		"items":[{"description":"Servicio","quantity":1,"subtotal":10,"itbms_amount":0.7,"total":10.7}],
		"subtotal":10,
		"itbms_total":0.7,
		"total_amount":10.7
	}`)
	if err := validateInvoiceJSON(ok); err != nil {
		t.Fatal(err)
	}
	bad := []byte(`{"issuer":{"name":"X"},"items":[{"description":"A","quantity":1,"total":1}],"total_amount":99}`)
	if err := validateInvoiceJSON(bad); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestStatusLabelOCR(t *testing.T) {
	ocr := NewOCRWorker(config.Config{})
	if got := StatusLabel(ocr, true); got != "enabled" {
		t.Fatalf("got %s", got)
	}
	if got := StatusLabel(ocr, false); got != "disabled" {
		t.Fatalf("got %s", got)
	}
	if got := StatusLabel(Stub{Kind: OCR}, true); got != "stub_enabled" {
		t.Fatalf("got %s", got)
	}
}
