package httpapi

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

func TestDecodeJSONBodyRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"email":"admin@example.com","password":"very-strong-password","extra":"nope"}`))

	var payload bootstrapRequest
	err := decodeJSONBody(req, &payload)
	if err == nil {
		t.Fatalf("decodeJSONBody() error = nil, want invalid json payload")
	}
	if err.Error() != "invalid json payload" {
		t.Fatalf("decodeJSONBody() error = %q, want invalid json payload", err.Error())
	}
}

func TestDecodeJSONBodyRejectsMultiplePayloads(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"email":"admin@example.com","password":"very-strong-password"}{"email":"other@example.com","password":"another-pass"}`))

	var payload bootstrapRequest
	err := decodeJSONBody(req, &payload)
	if err == nil {
		t.Fatalf("decodeJSONBody() error = nil, want invalid json payload")
	}
	if err.Error() != "invalid json payload" {
		t.Fatalf("decodeJSONBody() error = %q, want invalid json payload", err.Error())
	}
}
