package workers

import (
	"encoding/json"
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

func TestNormalizeDoubleCountedITBMS(t *testing.T) {
	raw := []byte(`{
		"invoice_number":"TFHKC20001788-00078976",
		"emission_date":"2026-03-20",
		"issuer":{"name":"PRICESMART PANAMA, S.A."},
		"items":[
			{"description":"Tinta Epson","quantity":1,"subtotal":19.99,"itbms_amount":1.40,"total":21.39},
			{"description":"Epson T664","quantity":1,"subtotal":17.49,"itbms_amount":1.22,"total":18.71}
		],
		"subtotal":37.48,
		"itbms_total":2.62,
		"total_amount":42.72
	}`)
	out, err := normalizeInvoiceJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var inv llmInvoice
	if err := json.Unmarshal(out, &inv); err != nil {
		t.Fatal(err)
	}
	if inv.TotalAmount < 40.05 || inv.TotalAmount > 40.15 {
		t.Fatalf("total_amount=%v want 40.10", inv.TotalAmount)
	}
	if inv.Items[0].UnitPrice < 19.98 || inv.Items[0].UnitPrice > 20.00 {
		t.Fatalf("unit_price=%v want 19.99 from subtotal", inv.Items[0].UnitPrice)
	}
	if inv.Items[1].UnitPrice < 17.48 || inv.Items[1].UnitPrice > 17.50 {
		t.Fatalf("unit_price=%v want 17.49 from subtotal", inv.Items[1].UnitPrice)
	}
}

func TestNormalizeKeepsUnitPrice(t *testing.T) {
	raw := []byte(`{
		"invoice_number":"A-1",
		"emission_date":"2026-09-01",
		"issuer":{"name":"Proveedor","ruc":"1"},
		"items":[{"description":"Servicio","quantity":2,"unit_price":5,"subtotal":10,"itbms_amount":0.7,"total":10.7}],
		"subtotal":10,
		"itbms_total":0.7,
		"total_amount":10.7
	}`)
	out, err := normalizeInvoiceJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var inv llmInvoice
	if err := json.Unmarshal(out, &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Items[0].UnitPrice != 5 {
		t.Fatalf("unit_price=%v", inv.Items[0].UnitPrice)
	}
	if inv.Issuer.RUC != "1" {
		t.Fatalf("issuer ruc dropped: %q", inv.Issuer.RUC)
	}
}

func TestNormalizeDropsNonPanamaDV(t *testing.T) {
	raw := []byte(`{
		"invoice_number":"FBADS-1",
		"emission_date":"2026-09-11",
		"issuer":{"name":"Meta Platforms Ireland Limited","ruc":"IE9692928F","dv":"Panamá"},
		"items":[{"description":"Ads","quantity":1,"unit_price":67,"subtotal":67,"itbms_amount":0,"total":67}],
		"subtotal":67,
		"itbms_total":0,
		"total_amount":67
	}`)
	out, err := normalizeInvoiceJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var inv llmInvoice
	if err := json.Unmarshal(out, &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Issuer.DV != "" {
		t.Fatalf("dv=%q want empty", inv.Issuer.DV)
	}
}

func TestSanitizePanamaDV(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"69", "69"},
		{"02", "02"},
		{"2", "2"},
		{"", ""},
		{"Panamá", ""},
		{"123", ""},
	}
	for _, c := range cases {
		if got := sanitizePanamaDV(c.in); got != c.want {
			t.Fatalf("sanitizePanamaDV(%q)=%q want %q", c.in, got, c.want)
		}
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
