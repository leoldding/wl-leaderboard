package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRow(t *testing.T) {
	tests := []struct {
		name string
		row  rowPayload
		want bool
	}{
		{name: "zero lifts", row: rowPayload{Name: "Ada", Snatch: 0, CleanJerk: 0}, want: true},
		{name: "two decimal lifts", row: rowPayload{Name: "Ada", Snatch: 123.45, CleanJerk: 500}, want: true},
		{name: "negative lift", row: rowPayload{Name: "Ada", Snatch: -0.01, CleanJerk: 100}},
		{name: "over maximum", row: rowPayload{Name: "Ada", Snatch: 500.01, CleanJerk: 100}},
		{name: "three decimal places", row: rowPayload{Name: "Ada", Snatch: 100.001, CleanJerk: 100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateRow(tt.row) == nil; got != tt.want {
				t.Errorf("validateRow() success = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeRowPayload(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     bool
	}{
		{name: "valid JSON", contentType: "application/json", body: `{"name":"Ada","snatch":100.25,"cleanjerk":120}`},
		{name: "unknown field", contentType: "application/json", body: `{"name":"Ada","snatch":100,"cleanjerk":120,"extra":true}`, wantErr: true},
		{name: "multiple JSON values", contentType: "application/json", body: `{} {}`, wantErr: true},
		{name: "wrong content type", contentType: "text/plain", body: `{}`, wantErr: true},
		{name: "body too large", contentType: "application/json", body: `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", tt.contentType)
			payload := rowPayload{}
			err := decodeRowPayload(httptest.NewRecorder(), r, &payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeRowPayload() error = %v, want error = %v", err, tt.wantErr)
			}
		})
	}
}
