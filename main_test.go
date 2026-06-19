package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
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

func TestParseCodexAuthJSONReadsProxyURL(t *testing.T) {
	payload, err := parseCodexAuthJSON(json.RawMessage(`{
		"access_token": "token",
		"email": "user@example.com",
		"account_id": "acc",
		"chatgpt_plan_type": "plus",
		"proxy_url": "http://127.0.0.1:7897"
	}`))
	if err != nil {
		t.Fatalf("parseCodexAuthJSON error = %v", err)
	}
	if payload.ProxyURL != "http://127.0.0.1:7897" {
		t.Fatalf("ProxyURL = %q", payload.ProxyURL)
	}
}

func TestParseCodexAuthJSONReadsAttributeProxyURL(t *testing.T) {
	payload, err := parseCodexAuthJSON(json.RawMessage(`{
		"token_data": {"access_token": "token"},
		"attributes": {"proxy_url": "socks5://127.0.0.1:7897"}
	}`))
	if err != nil {
		t.Fatalf("parseCodexAuthJSON error = %v", err)
	}
	if payload.ProxyURL != "socks5://127.0.0.1:7897" {
		t.Fatalf("ProxyURL = %q", payload.ProxyURL)
	}
}

func TestCodexQuotaHeaders(t *testing.T) {
	headers := codexQuotaHeaders("token", " account ")
	if headers.Get("authorization") != "Bearer token" {
		t.Fatalf("authorization = %q", headers.Get("authorization"))
	}
	if headers.Get("chatgpt-account-id") != "account" {
		t.Fatalf("chatgpt-account-id = %q", headers.Get("chatgpt-account-id"))
	}
	if headers.Get("originator") != defaultCodexOriginator {
		t.Fatalf("originator = %q", headers.Get("originator"))
	}
	if headers.Get("user-agent") != defaultCodexUserAgent {
		t.Fatalf("user-agent = %q", headers.Get("user-agent"))
	}
	if headers.Get("openai-beta") == "" {
		t.Fatal("openai-beta header is empty")
	}
}

func TestQuotaSourceDefaultsToCPAAPICall(t *testing.T) {
	if got := quotaSource(nil); got != quotaSourceCPAAPICall {
		t.Fatalf("quotaSource(nil) = %q", got)
	}
	if got := quotaSource(url.Values{}); got != quotaSourceCPAAPICall {
		t.Fatalf("quotaSource(empty) = %q", got)
	}
	if got := quotaSource(url.Values{"source": []string{"upstream"}}); got != quotaSourceUpstream {
		t.Fatalf("quotaSource(upstream) = %q", got)
	}
	if got := quotaSource(url.Values{"source": []string{"runtime"}}); got != quotaSourceCPARuntime {
		t.Fatalf("quotaSource(runtime) = %q", got)
	}
	if got := quotaSource(url.Values{"endpoint": []string{defaultQuotaEndpoint}}); got != quotaSourceCPAAPICall {
		t.Fatalf("quotaSource(endpoint) = %q", got)
	}
}

func TestCPAAPICallURL(t *testing.T) {
	got, err := cpaAPICallURL(url.Values{"cpa_base_url": []string{"http://127.0.0.1:9999/base/"}})
	if err != nil {
		t.Fatalf("cpaAPICallURL error = %v", err)
	}
	if got != "http://127.0.0.1:9999/base/v0/management/api-call" {
		t.Fatalf("cpaAPICallURL = %q", got)
	}
}

func TestCPAManagementHeaders(t *testing.T) {
	headers := cpaManagementHeaders(http.Header{
		"Authorization":    []string{"Bearer key"},
		"X-Management-Key": []string{"mgmt"},
		"Cookie":           []string{"ignored=true"},
	})
	if headers.Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q", headers.Get("Content-Type"))
	}
	if headers.Get("Authorization") != "Bearer key" {
		t.Fatalf("Authorization = %q", headers.Get("Authorization"))
	}
	if headers.Get("X-Management-Key") != "mgmt" {
		t.Fatalf("X-Management-Key = %q", headers.Get("X-Management-Key"))
	}
	if headers.Get("Cookie") != "" {
		t.Fatalf("Cookie = %q, want empty", headers.Get("Cookie"))
	}
}

func TestQuotaFromCPARuntimeAuth(t *testing.T) {
	nextRetry := time.Now().Add(time.Minute).UTC()
	result := quotaFromCPARuntimeAuth(pluginapi.HostAuthFileEntry{
		ID:             "id",
		AuthIndex:      "auth-index",
		Name:           "codex.json",
		Provider:       "codex",
		Email:          "user@example.com",
		Account:        "account",
		AccountType:    "plus",
		Status:         "ready",
		StatusMessage:  "quota exhausted",
		Unavailable:    true,
		NextRetryAfter: nextRetry,
		Success:        3,
		Failed:         2,
		RecentRequests: []pluginapi.HostRecentRequestEntry{{Success: 1, Failed: 1}},
	})

	if result.OK {
		t.Fatal("OK = true, want false for unavailable auth")
	}
	if result.Source != quotaSourceCPARuntime {
		t.Fatalf("Source = %q", result.Source)
	}
	if result.Endpoint != cpaRuntimeEndpoint {
		t.Fatalf("Endpoint = %q", result.Endpoint)
	}
	if result.ErrorCode != "auth_unavailable" {
		t.Fatalf("ErrorCode = %q", result.ErrorCode)
	}
	if !json.Valid(result.Quota) {
		t.Fatalf("Quota is not valid JSON: %s", string(result.Quota))
	}
	if got, ok := result.Fields["quota_exceeded"].(bool); !ok || !got {
		t.Fatalf("quota_exceeded = %#v", result.Fields["quota_exceeded"])
	}
	if result.NextRetryAfter == nil || !result.NextRetryAfter.Equal(nextRetry) {
		t.Fatalf("NextRetryAfter = %#v", result.NextRetryAfter)
	}
	if _, ok := result.Fields["recent_requests"]; ok {
		t.Fatal("Fields contains duplicated recent_requests")
	}
	if result.Raw != nil {
		t.Fatalf("Raw = %#v, want nil for CPA runtime result", result.Raw)
	}
}

func TestQuotaFromCPARuntimeAuthOmitsZeroRetryTime(t *testing.T) {
	result := quotaFromCPARuntimeAuth(pluginapi.HostAuthFileEntry{
		AuthIndex: "auth-index",
		Name:      "codex.json",
		Provider:  "codex",
		Status:    "active",
	})
	if result.NextRetryAfter != nil {
		t.Fatalf("NextRetryAfter = %#v, want nil", result.NextRetryAfter)
	}
	raw := jsonResponse(http.StatusOK, quotaResponse{
		Provider:  "codex",
		Source:    quotaSourceCPARuntime,
		Endpoint:  cpaRuntimeEndpoint,
		CheckedAt: "2026-06-20T00:00:00Z",
		Accounts:  []quotaAccountResult{result},
		Summary:   quotaSummary{Total: 1, OK: 1},
	})
	if strings.Contains(string(raw.Body), "0001-01-01") {
		t.Fatalf("response leaked zero time: %s", string(raw.Body))
	}
}
