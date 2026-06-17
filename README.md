# Codex Quota CLIProxyAPI Plugin

Minimal CLIProxyAPI native plugin for reading Codex quota information through the Management API.

It only registers one Management API route:

```text
GET /v0/management/codex-quota
GET /v0/management/codex/quota
```

Optional query parameters:

- `auth_index`: query one Codex auth by runtime index. If omitted, all Codex auths are queried.
- `endpoint`: override the ChatGPT backend quota endpoint. If omitted, the plugin tries a small default candidate list: `/backend-api/codex/quota`, `/backend-api/codex/usage_limits`, then `/backend-api/codex/usage`.

Both routes return the same JSON. The response keeps both normalized fields and the upstream raw response so clients such as codexbar and cc-switch can consume whichever shape they need.

## Build

```bash
make build
```

The output is `codex-quota.so`, `codex-quota.dylib`, or `codex-quota.dll` depending on the target OS.

## Configure

Copy the built library into your CLIProxyAPI plugin directory and enable plugins:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    codex-quota:
      enabled: true
```

Then request:

```bash
curl -H "Authorization: Bearer <management-token>" \
  "http://127.0.0.1:8080/v0/management/codex-quota"
```

The plugin uses CLIProxyAPI host callbacks to read Codex auth records and to issue the upstream HTTP request through the host HTTP client.
