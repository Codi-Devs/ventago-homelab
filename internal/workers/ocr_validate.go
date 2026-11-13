package workers

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const invoiceAmountTolerance = 0.05

type llmInvoice struct {
	InvoiceNumber string     `json:"invoice_number"`
	EmissionDate  string     `json:"emission_date"`
	Issuer        llmParty   `json:"issuer"`
	Receiver      *llmParty  `json:"receiver,omitempty"`
	Items         []llmItem  `json:"items"`
	Subtotal      float64    `json:"subtotal"`
	ITBMSTotal    float64    `json:"itbms_total"`
	TotalAmount   float64    `json:"total_amount"`
	Notes         string     `json:"notes,omitempty"`
	CUFE          string     `json:"cufe,omitempty"`
}

type llmParty struct {
	Name    string `json:"name"`
	RUC     string `json:"ruc,omitempty"`
	DV      string `json:"dv,omitempty"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
}

type llmItem struct {
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	UnitPrice      float64 `json:"unit_price"`
	DiscountAmount float64 `json:"discount_amount"`
	ITBMSAmount    float64 `json:"itbms_amount"`
	Subtotal       float64 `json:"subtotal"`
	Total          float64 `json:"total"`
}

func itemsLineTotal(inv llmInvoice) float64 {
	var itemsTotal float64
	for _, item := range inv.Items {
		itemsTotal += item.Total
	}
	return itemsTotal
}

func fillMissingUnitPrices(inv *llmInvoice) {
	for i := range inv.Items {
		item := &inv.Items[i]
		if item.UnitPrice > 0 || item.Quantity <= 0 || item.Subtotal <= 0 {
			continue
		}
		item.UnitPrice = item.Subtotal / item.Quantity
	}
}

// qwen often adds ITBMS on top of an already-taxed TOTAL (40.10 + 2.62 = 42.72
// on a PriceSmart ticket whose printed TOTAL is 40.10).
func coerceDoubleCountedITBMS(inv *llmInvoice) {
	if inv.ITBMSTotal <= 0 {
		return
	}
	itemsTotal := itemsLineTotal(*inv)
	if math.Abs(inv.TotalAmount-(itemsTotal+inv.ITBMSTotal)) <= invoiceAmountTolerance {
		inv.TotalAmount = itemsTotal
	}
	if inv.Subtotal > 0 {
		printed := inv.Subtotal + inv.ITBMSTotal
		if math.Abs(itemsTotal-printed) <= invoiceAmountTolerance {
			inv.TotalAmount = printed
		}
	}
}

func normalizeInvoiceJSON(raw []byte) ([]byte, error) {
	var inv llmInvoice
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, fmt.Errorf("ocr_llm_invalid_json: %w", err)
	}
	coerceDoubleCountedITBMS(&inv)
	fillMissingUnitPrices(&inv)
	inv.Issuer.DV = sanitizePanamaDV(inv.Issuer.DV)
	if inv.Receiver != nil {
		inv.Receiver.DV = sanitizePanamaDV(inv.Receiver.DV)
	}
	out, err := json.Marshal(inv)
	if err != nil {
		return nil, err
	}
	if err := validateInvoiceJSON(out); err != nil {
		return nil, err
	}
	return out, nil
}

func validateInvoiceJSON(raw []byte) error {
	var inv llmInvoice
	if err := json.Unmarshal(raw, &inv); err != nil {
		return fmt.Errorf("ocr_llm_invalid_json: %w", err)
	}
	if strings.TrimSpace(inv.Issuer.Name) == "" {
		return fmt.Errorf("ocr_llm_missing_issuer")
	}
	if len(inv.Items) == 0 {
		return fmt.Errorf("ocr_llm_missing_items")
	}
	if inv.TotalAmount <= 0 {
		return fmt.Errorf("ocr_llm_invalid_total")
	}
	var itemsTotal, itemsSub, itemsTax float64
	for i, item := range inv.Items {
		if strings.TrimSpace(item.Description) == "" {
			return fmt.Errorf("ocr_llm_item_%d_missing_description", i)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("ocr_llm_item_%d_invalid_quantity", i)
		}
		itemsTotal += item.Total
		itemsSub += item.Subtotal
		itemsTax += item.ITBMSAmount
	}
	if math.Abs(itemsTotal-inv.TotalAmount) > invoiceAmountTolerance {
		return fmt.Errorf("ocr_llm_totals_mismatch: items_total=%.4f invoice_total=%.4f", itemsTotal, inv.TotalAmount)
	}
	if inv.Subtotal > 0 && math.Abs(itemsSub-inv.Subtotal) > invoiceAmountTolerance {
		return fmt.Errorf("ocr_llm_subtotal_mismatch")
	}
	if inv.ITBMSTotal > 0 && math.Abs(itemsTax-inv.ITBMSTotal) > invoiceAmountTolerance {
		return fmt.Errorf("ocr_llm_itbms_mismatch")
	}
	return nil
}

// Panama RUC check digit: 1-2 numeric digits. Empty for foreign issuers.
func sanitizePanamaDV(raw string) string {
	dv := strings.TrimSpace(raw)
	if dv == "" {
		return ""
	}
	runes := []rune(dv)
	if len(runes) > 2 {
		return ""
	}
	for _, r := range runes {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return dv
}
