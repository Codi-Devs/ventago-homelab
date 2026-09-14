package workers

import "encoding/json"

var invoiceJSONSchema = json.RawMessage(`{
  "type": "object",
  "required": ["invoice_number", "emission_date", "issuer", "items", "subtotal", "itbms_total", "total_amount"],
  "additionalProperties": false,
  "properties": {
    "invoice_number": {"type": "string"},
    "emission_date": {"type": "string"},
    "cufe": {"type": "string"},
    "notes": {"type": "string"},
    "issuer": {
      "type": "object",
      "required": ["name", "ruc", "dv", "address", "phone"],
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string"},
        "ruc": {"type": "string"},
        "dv": {"type": "string"},
        "address": {"type": "string"},
        "phone": {"type": "string"}
      }
    },
    "receiver": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string"},
        "ruc": {"type": "string"},
        "dv": {"type": "string"},
        "address": {"type": "string"},
        "phone": {"type": "string"}
      }
    },
    "items": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "required": ["description", "quantity", "unit_price", "discount_amount", "itbms_amount", "subtotal", "total"],
        "additionalProperties": false,
        "properties": {
          "description": {"type": "string"},
          "quantity": {"type": "number"},
          "unit_price": {"type": "number"},
          "discount_amount": {"type": "number"},
          "itbms_amount": {"type": "number"},
          "subtotal": {"type": "number"},
          "total": {"type": "number"}
        }
      }
    },
    "subtotal": {"type": "number"},
    "itbms_total": {"type": "number"},
    "total_amount": {"type": "number"}
  }
}`)
