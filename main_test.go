package main

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestQuotaHTTPErrorSummarizesCloudflareChallenge(t *testing.T) {
	body := []byte(`<html><span id="challenge-error-text">Enable JavaScript and cookies to continue</span><script>window._cf_chl_opt={}</script></html>`)
	resp := hostHTTPResponse{StatusCode: http.StatusForbidden, Body: body}

	if !shouldTryNextEndpoint(resp) {
		t.Fatal("shouldTryNextEndpoint = false, want true for Cloudflare challenge")
	}
	errText := quotaHTTPError(resp)
	if !strings.Contains(errText, "Cloudflare challenge") {
		t.Fatalf("quotaHTTPError = %q, want Cloudflare challenge summary", errText)
	}
	if strings.Contains(errText, "_cf_chl_opt") {
		t.Fatalf("quotaHTTPError leaked challenge HTML: %q", errText)
	}
	if code := quotaHTTPErrorCode(resp); code != "cloudflare_challenge" {
		t.Fatalf("quotaHTTPErrorCode = %q, want cloudflare_challenge", code)
	}
}

func TestTrimmedResponseTextCapsLargeHTML(t *testing.T) {
	text := trimmedResponseText([]byte(strings.Repeat("x", maxRawTextLength+100)))
	if len(text) > maxRawTextLength+len("...(truncated)") {
		t.Fatalf("trimmed text length = %d, want capped", len(text))
	}
	if !strings.HasSuffix(text, "...(truncated)") {
		t.Fatalf("trimmed text suffix = %q, want truncation marker", text[len(text)-20:])
	}
}
