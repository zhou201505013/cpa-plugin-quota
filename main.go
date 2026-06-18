package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct {
	void* ptr;
	size_t len;
} cliproxy_buffer;

typedef int (*cliproxy_host_call_fn)(void*, const char*, const uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_host_free_fn)(void*, size_t);

typedef struct {
	uint32_t abi_version;
	void* host_ctx;
	cliproxy_host_call_fn call;
	cliproxy_host_free_fn free_buffer;
} cliproxy_host_api;

typedef int (*cliproxy_plugin_call_fn)(char*, uint8_t*, size_t, cliproxy_buffer*);
typedef void (*cliproxy_plugin_free_fn)(void*, size_t);
typedef void (*cliproxy_plugin_shutdown_fn)(void);

typedef struct {
	uint32_t abi_version;
	cliproxy_plugin_call_fn call;
	cliproxy_plugin_free_fn free_buffer;
	cliproxy_plugin_shutdown_fn shutdown;
} cliproxy_plugin_api;

extern int codexQuotaPluginCall(char*, uint8_t*, size_t, cliproxy_buffer*);
extern void codexQuotaPluginFree(void*, size_t);
extern void codexQuotaPluginShutdown(void);

static const cliproxy_host_api* stored_host;

static void store_host_api(const cliproxy_host_api* host) {
	stored_host = host;
}

static int call_host_api(const char* method, const uint8_t* request, size_t request_len, cliproxy_buffer* response) {
	if (stored_host == NULL || stored_host->call == NULL) {
		return 1;
	}
	return stored_host->call(stored_host->host_ctx, method, request, request_len, response);
}

static void free_host_buffer(void* ptr, size_t len) {
	if (stored_host != NULL && stored_host->free_buffer != NULL && ptr != NULL) {
		stored_host->free_buffer(ptr, len);
	}
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	pluginName           = "codex-quota"
	defaultQuotaEndpoint = "https://chatgpt.com/backend-api/codex/quota"
)

var pluginVersion = "0.1.0"
var defaultQuotaEndpoints = []string{
	defaultQuotaEndpoint,
	"https://chatgpt.com/backend-api/codex/usage_limits",
	"https://chatgpt.com/backend-api/codex/usage",
}

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32                   `json:"schema_version"`
	Metadata      pluginapi.Metadata       `json:"metadata"`
	Capabilities  registrationCapabilities `json:"capabilities"`
}

type registrationCapabilities struct {
	ManagementAPI bool `json:"management_api"`
}

type managementRegistration struct {
	Routes []managementRoute `json:"routes,omitempty"`
}

type managementRoute struct {
	Method      string `json:"Method"`
	Path        string `json:"Path"`
	Description string `json:"Description,omitempty"`
}

type managementRequest struct {
	Method         string      `json:"Method"`
	Path           string      `json:"Path"`
	Headers        http.Header `json:"Headers"`
	Query          url.Values  `json:"Query"`
	Body           []byte      `json:"Body"`
	HostCallbackID string      `json:"host_callback_id,omitempty"`
}

type managementResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

type hostAuthListResponse struct {
	Files []pluginapi.HostAuthFileEntry `json:"files"`
}

type hostAuthGetResponse struct {
	AuthIndex string          `json:"auth_index"`
	Name      string          `json:"name,omitempty"`
	Path      string          `json:"path,omitempty"`
	JSON      json.RawMessage `json:"json"`
}

type hostHTTPRequest struct {
	HostCallbackID string      `json:"host_callback_id,omitempty"`
	Method         string      `json:"method,omitempty"`
	URL            string      `json:"url,omitempty"`
	Headers        http.Header `json:"headers,omitempty"`
	Body           []byte      `json:"body,omitempty"`
}

type hostHTTPResponse struct {
	StatusCode int         `json:"StatusCode"`
	Headers    http.Header `json:"Headers"`
	Body       []byte      `json:"Body"`
}

type quotaResponse struct {
	Provider  string               `json:"provider"`
	Endpoint  string               `json:"endpoint"`
	Endpoints []string             `json:"endpoints,omitempty"`
	CheckedAt string               `json:"checked_at"`
	Accounts  []quotaAccountResult `json:"accounts"`
	Summary   quotaSummary         `json:"summary"`
}

type quotaSummary struct {
	Total int `json:"total"`
	OK    int `json:"ok"`
	Error int `json:"error"`
}

type quotaAccountResult struct {
	AuthIndex  string          `json:"auth_index,omitempty"`
	ID         string          `json:"id,omitempty"`
	Name       string          `json:"name,omitempty"`
	Email      string          `json:"email,omitempty"`
	AccountID  string          `json:"account_id,omitempty"`
	PlanType   string          `json:"plan_type,omitempty"`
	Status     string          `json:"status,omitempty"`
	OK         bool            `json:"ok"`
	HTTPStatus int             `json:"http_status,omitempty"`
	Endpoint   string          `json:"endpoint,omitempty"`
	Quota      json.RawMessage `json:"quota,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
	Fields     map[string]any  `json:"fields,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type codexAuthPayload struct {
	AccessToken string
	Email       string
	AccountID   string
	PlanType    string
}

func main() {}

//export cliproxy_plugin_init
func cliproxy_plugin_init(host *C.cliproxy_host_api, plugin *C.cliproxy_plugin_api) C.int {
	if plugin == nil {
		return 1
	}
	C.store_host_api(host)
	plugin.abi_version = C.uint32_t(pluginabi.ABIVersion)
	plugin.call = C.cliproxy_plugin_call_fn(C.codexQuotaPluginCall)
	plugin.free_buffer = C.cliproxy_plugin_free_fn(C.codexQuotaPluginFree)
	plugin.shutdown = C.cliproxy_plugin_shutdown_fn(C.codexQuotaPluginShutdown)
	return 0
}

//export codexQuotaPluginCall
func codexQuotaPluginCall(method *C.char, request *C.uint8_t, requestLen C.size_t, response *C.cliproxy_buffer) C.int {
	if response != nil {
		response.ptr = nil
		response.len = 0
	}
	if method == nil {
		writeResponse(response, errorEnvelope("invalid_method", "method is required"))
		return 1
	}
	var requestBytes []byte
	if request != nil && requestLen > 0 {
		requestBytes = C.GoBytes(unsafe.Pointer(request), C.int(requestLen))
	}
	raw, errHandle := handleMethod(C.GoString(method), requestBytes)
	if errHandle != nil {
		writeResponse(response, errorEnvelope("plugin_error", errHandle.Error()))
		return 1
	}
	writeResponse(response, raw)
	return 0
}

//export codexQuotaPluginFree
func codexQuotaPluginFree(ptr unsafe.Pointer, len C.size_t) {
	if ptr != nil {
		C.free(ptr)
	}
	_ = len
}

//export codexQuotaPluginShutdown
func codexQuotaPluginShutdown() {}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodManagementRegister:
		return okEnvelope(managementRegistration{
			Routes: []managementRoute{
				{
					Method:      http.MethodGet,
					Path:        "/codex-quota",
					Description: "Returns Codex quota information for loaded Codex auths.",
				},
				{
					Method:      http.MethodGet,
					Path:        "/codex/quota",
					Description: "Alias for /codex-quota.",
				},
			},
		})
	case pluginabi.MethodManagementHandle:
		return handleManagement(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             pluginName,
			Version:          pluginVersion,
			Author:           "zhou201505013",
			GitHubRepository: "https://github.com/zhou201505013/cpa-plugin-quota",
			ConfigFields:     []pluginapi.ConfigField{},
		},
		Capabilities: registrationCapabilities{ManagementAPI: true},
	}
}

func handleManagement(raw []byte) ([]byte, error) {
	var req managementRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return nil, fmt.Errorf("decode management request: %w", errUnmarshal)
		}
	}
	if req.Method != "" && !strings.EqualFold(req.Method, http.MethodGet) {
		return okEnvelope(jsonResponse(http.StatusMethodNotAllowed, map[string]any{
			"error": "method_not_allowed",
		}))
	}
	resp := queryCodexQuota(req)
	status := http.StatusOK
	if resp.Summary.Total == 0 {
		status = http.StatusNotFound
	}
	return okEnvelope(jsonResponse(status, resp))
}

func queryCodexQuota(req managementRequest) quotaResponse {
	endpoints := quotaEndpoints(req.Query)
	out := quotaResponse{
		Provider:  "codex",
		Endpoint:  endpoints[0],
		Endpoints: endpoints,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	auths, errList := listCodexAuths()
	if errList != nil {
		out.Accounts = append(out.Accounts, quotaAccountResult{
			OK:    false,
			Error: errList.Error(),
		})
		out.Summary.Total = 1
		out.Summary.Error = 1
		return out
	}
	authIndexFilter := strings.TrimSpace(req.Query.Get("auth_index"))
	for _, auth := range auths {
		if authIndexFilter != "" && auth.AuthIndex != authIndexFilter {
			continue
		}
		result := queryOneCodexQuota(auth, endpoints, req.HostCallbackID)
		out.Accounts = append(out.Accounts, result)
		if result.OK {
			out.Summary.OK++
		} else {
			out.Summary.Error++
		}
	}
	out.Summary.Total = len(out.Accounts)
	return out
}

func quotaEndpoints(query url.Values) []string {
	if query != nil {
		if endpoint := strings.TrimSpace(query.Get("endpoint")); endpoint != "" {
			return []string{endpoint}
		}
	}
	return append([]string(nil), defaultQuotaEndpoints...)
}

func listCodexAuths() ([]pluginapi.HostAuthFileEntry, error) {
	result, errCall := callHost(pluginabi.MethodHostAuthList, nil)
	if errCall != nil {
		return nil, errCall
	}
	var resp hostAuthListResponse
	if errUnmarshal := json.Unmarshal(result, &resp); errUnmarshal != nil {
		return nil, fmt.Errorf("decode host.auth.list result: %w", errUnmarshal)
	}
	var auths []pluginapi.HostAuthFileEntry
	for _, entry := range resp.Files {
		provider := strings.TrimSpace(entry.Provider)
		if provider == "" {
			provider = strings.TrimSpace(entry.Type)
		}
		if strings.EqualFold(provider, "codex") {
			auths = append(auths, entry)
		}
	}
	sort.Slice(auths, func(i, j int) bool {
		return strings.ToLower(auths[i].Name) < strings.ToLower(auths[j].Name)
	})
	return auths, nil
}

func queryOneCodexQuota(auth pluginapi.HostAuthFileEntry, endpoints []string, hostCallbackID string) quotaAccountResult {
	if len(endpoints) == 0 {
		endpoints = []string{defaultQuotaEndpoint}
	}
	result := quotaAccountResult{
		AuthIndex: auth.AuthIndex,
		ID:        auth.ID,
		Name:      auth.Name,
		Email:     auth.Email,
		AccountID: auth.Account,
		PlanType:  auth.AccountType,
		Status:    auth.Status,
	}
	authPayload, errAuth := loadCodexAuthPayload(auth)
	if errAuth != nil {
		result.Error = errAuth.Error()
		return result
	}
	if result.Email == "" {
		result.Email = authPayload.Email
	}
	if result.AccountID == "" {
		result.AccountID = authPayload.AccountID
	}
	if result.PlanType == "" {
		result.PlanType = authPayload.PlanType
	}
	if authPayload.AccessToken == "" {
		result.Error = "codex auth has no access_token"
		return result
	}

	var httpResp hostHTTPResponse
	var errHTTP error
	for i, endpoint := range endpoints {
		result.Endpoint = endpoint
		httpResp, errHTTP = doQuotaHTTPRequest(endpoint, authPayload.AccessToken, hostCallbackID)
		if errHTTP != nil {
			result.Error = errHTTP.Error()
			return result
		}
		if httpResp.StatusCode != http.StatusNotFound && httpResp.StatusCode != http.StatusMethodNotAllowed {
			break
		}
		if i == len(endpoints)-1 {
			break
		}
	}
	result.HTTPStatus = httpResp.StatusCode
	result.Raw = append(json.RawMessage(nil), httpResp.Body...)
	if len(httpResp.Body) > 0 && json.Valid(httpResp.Body) {
		result.Quota = append(json.RawMessage(nil), httpResp.Body...)
		result.Fields = extractQuotaFields(httpResp.Body)
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		result.Error = strings.TrimSpace(string(httpResp.Body))
		if result.Error == "" {
			result.Error = "quota endpoint returned status " + strconv.Itoa(httpResp.StatusCode)
		}
		return result
	}
	result.OK = true
	return result
}

func loadCodexAuthPayload(auth pluginapi.HostAuthFileEntry) (codexAuthPayload, error) {
	if strings.TrimSpace(auth.AuthIndex) == "" {
		return codexAuthPayload{}, fmt.Errorf("codex auth %q has no auth_index", auth.Name)
	}
	result, errCall := callHost(pluginabi.MethodHostAuthGet, pluginapi.HostAuthGetRequest{AuthIndex: auth.AuthIndex})
	if errCall != nil {
		return codexAuthPayload{}, errCall
	}
	var resp hostAuthGetResponse
	if errUnmarshal := json.Unmarshal(result, &resp); errUnmarshal != nil {
		return codexAuthPayload{}, fmt.Errorf("decode host.auth.get result: %w", errUnmarshal)
	}
	return parseCodexAuthJSON(resp.JSON)
}

func parseCodexAuthJSON(raw json.RawMessage) (codexAuthPayload, error) {
	var root map[string]any
	if errUnmarshal := json.Unmarshal(raw, &root); errUnmarshal != nil {
		return codexAuthPayload{}, fmt.Errorf("decode codex auth json: %w", errUnmarshal)
	}
	payload := codexAuthPayload{
		AccessToken: getString(root, "access_token"),
		Email:       getString(root, "email"),
		AccountID:   getString(root, "account_id"),
		PlanType:    getString(root, "chatgpt_plan_type"),
	}
	if tokenData, ok := root["token_data"].(map[string]any); ok {
		if payload.AccessToken == "" {
			payload.AccessToken = getString(tokenData, "access_token")
		}
		if payload.Email == "" {
			payload.Email = getString(tokenData, "email")
		}
		if payload.AccountID == "" {
			payload.AccountID = getString(tokenData, "account_id")
		}
	}
	if payload.PlanType == "" {
		payload.PlanType = getString(root, "plan_type")
	}
	return payload, nil
}

func doQuotaHTTPRequest(endpoint, accessToken, hostCallbackID string) (hostHTTPResponse, error) {
	req := hostHTTPRequest{
		HostCallbackID: strings.TrimSpace(hostCallbackID),
		Method:         http.MethodGet,
		URL:            endpoint,
		Headers: http.Header{
			"accept":        []string{"application/json"},
			"authorization": []string{"Bearer " + accessToken},
			"user-agent":    []string{"codex-quota-cli-proxy-plugin/" + pluginVersion},
		},
	}
	result, errCall := callHost(pluginabi.MethodHostHTTPDo, req)
	if errCall != nil {
		return hostHTTPResponse{}, errCall
	}
	var resp hostHTTPResponse
	if errUnmarshal := json.Unmarshal(result, &resp); errUnmarshal != nil {
		return hostHTTPResponse{}, fmt.Errorf("decode host.http.do result: %w", errUnmarshal)
	}
	return resp, nil
}

func extractQuotaFields(raw json.RawMessage) map[string]any {
	var v any
	if errUnmarshal := json.Unmarshal(raw, &v); errUnmarshal != nil {
		return nil
	}
	fields := map[string]any{}
	flattenQuotaFields(fields, "", v)
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func flattenQuotaFields(out map[string]any, prefix string, v any) {
	switch value := v.(type) {
	case map[string]any:
		for key, child := range value {
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			if shouldExposeQuotaField(key) {
				out[next] = child
			}
			flattenQuotaFields(out, next, child)
		}
	case []any:
		for i, child := range value {
			next := prefix + "[" + strconv.Itoa(i) + "]"
			flattenQuotaFields(out, next, child)
		}
	}
}

func shouldExposeQuotaField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return false
	}
	for _, needle := range []string{
		"quota", "limit", "remaining", "reset", "usage", "used", "plan",
		"window", "period", "expires", "cap", "count", "model",
	} {
		if strings.Contains(key, needle) {
			return true
		}
	}
	return false
}

func callHost(method string, payload any) (json.RawMessage, error) {
	var rawPayload []byte
	var errMarshal error
	if payload != nil {
		rawPayload, errMarshal = json.Marshal(payload)
		if errMarshal != nil {
			return nil, fmt.Errorf("marshal host callback payload %s: %w", method, errMarshal)
		}
	}

	cMethod := C.CString(method)
	defer C.free(unsafe.Pointer(cMethod))

	var response C.cliproxy_buffer
	var requestPtr *C.uint8_t
	if len(rawPayload) > 0 {
		cPayload := C.CBytes(rawPayload)
		if cPayload == nil {
			return nil, fmt.Errorf("allocate host callback payload %s", method)
		}
		defer C.free(cPayload)
		requestPtr = (*C.uint8_t)(cPayload)
	}

	callCode := C.call_host_api(cMethod, requestPtr, C.size_t(len(rawPayload)), &response)
	var rawResponse []byte
	if response.ptr != nil && response.len > 0 {
		rawResponse = C.GoBytes(response.ptr, C.int(response.len))
	}
	if response.ptr != nil {
		C.free_host_buffer(response.ptr, response.len)
	}
	if len(rawResponse) == 0 {
		return nil, fmt.Errorf("host callback %s returned no response, code=%d", method, int(callCode))
	}

	var env envelope
	if errUnmarshal := json.Unmarshal(rawResponse, &env); errUnmarshal != nil {
		return nil, fmt.Errorf("decode host callback envelope %s: %w", method, errUnmarshal)
	}
	if !env.OK {
		if env.Error != nil {
			return nil, fmt.Errorf("%s: %s", env.Error.Code, env.Error.Message)
		}
		return nil, fmt.Errorf("host callback %s failed", method)
	}
	if callCode != 0 {
		return nil, fmt.Errorf("host callback %s returned code=%d", method, int(callCode))
	}
	return append(json.RawMessage(nil), env.Result...), nil
}

func jsonResponse(statusCode int, body any) managementResponse {
	raw, errMarshal := json.MarshalIndent(body, "", "  ")
	if errMarshal != nil {
		raw = []byte(`{"error":"marshal_response_failed"}`)
		statusCode = http.StatusInternalServerError
	}
	return managementResponse{
		StatusCode: statusCode,
		Headers: http.Header{
			"content-type": []string{"application/json; charset=utf-8"},
		},
		Body: raw,
	}
}

func okEnvelope(v any) ([]byte, error) {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

func writeResponse(response *C.cliproxy_buffer, raw []byte) {
	if response == nil || len(raw) == 0 {
		return
	}
	ptr := C.CBytes(raw)
	if ptr == nil {
		return
	}
	response.ptr = ptr
	response.len = C.size_t(len(raw))
}

func getString(root map[string]any, key string) string {
	value, ok := root[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}
