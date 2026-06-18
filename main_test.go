package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestJSONResponseAllowsTextRawBody(t *testing.T) {
	resp := quotaResponse{
		Provider:  "codex",
		Endpoint:  defaultQuotaEndpoint,
		CheckedAt: "2026-06-18T00:00:00Z",
		Accounts: []quotaAccountResult{{
			OK:         false,
			HTTPStatus: http.StatusForbidden,
			Raw:        "<html>forbidden</html>",
			Error:      "<html>forbidden</html>",
		}},
		Summary: quotaSummary{Total: 1, Error: 1},
	}

	out := jsonResponse(http.StatusOK, resp)
	if out.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", out.StatusCode, http.StatusOK)
	}
	if string(out.Body) == `{"error":"marshal_response_failed"}` {
		t.Fatal("jsonResponse failed to marshal text raw body")
	}
	var decoded quotaResponse
	if err := json.Unmarshal(out.Body, &decoded); err != nil {
		t.Fatalf("response body is not JSON: %v", err)
	}
	if decoded.Accounts[0].Raw != "<html>forbidden</html>" {
		t.Fatalf("Raw = %#v, want text body", decoded.Accounts[0].Raw)
	}
}
