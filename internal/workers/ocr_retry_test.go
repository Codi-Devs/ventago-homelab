package workers

import "testing"

func TestIsTransientAPIError(t *testing.T) {
	if isTransientAPIError(nil) {
		t.Fatal("nil")
	}
	cases := []struct {
		msg       string
		transient bool
	}{
		{`POST /api/v1/internal/homelab/ocr/2/complete: HTTP 500`, true},
		{`GET /lease: HTTP 401`, true},
		{`Post "http://192.168.40.165:5001/complete": dial tcp 192.168.40.165:5001: connect: connection refused`, true},
		{`POST /ocr/2/complete: HTTP 400`, false},
		{`complete failed`, false},
		{`i/o timeout`, true},
	}
	for _, c := range cases {
		got := isTransientAPIError(statusError(c.msg))
		if got != c.transient {
			t.Fatalf("%q transient=%v want %v", c.msg, got, c.transient)
		}
	}
}

type statusError string

func (e statusError) Error() string { return string(e) }

func TestThermalLimitIs88C(t *testing.T) {
	if thermalLimitMilliC != 88000 {
		t.Fatalf("thermalLimitMilliC=%d", thermalLimitMilliC)
	}
}
