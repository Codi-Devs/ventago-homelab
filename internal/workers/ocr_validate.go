package workers

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const invoiceAmountTolerance = 0.05

type llmInvoice struct {
	InvoiceNumber string    `json:"invoice_number"`
	EmissionDate  string    `json:"emission_date"`
	Issuer        llmParty  `json:"issuer"`
	Items         []llmItem `json:"items"`
	Subtotal      float64   `json:"subtotal"`
	ITBMSTotal    float64   `json:"itbms_total"`
	TotalAmount   float64   `json:"total_amount"`
	CUFE          string    `json:"cufe"`
}

type llmParty struct {
	Name string `json:"name"`
}

type llmItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Subtotal    float64 `json:"subtotal"`
	ITBMSAmount float64 `json:"itbms_amount"`
	Total       float64 `json:"total"`
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
