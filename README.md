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

## Publish for the CPA plugin store

This repository follows the same release asset convention as `cpa-plugin-codex-invite`.
The CPA plugin store can discover/install the plugin from GitHub releases when each
release contains platform zip packages and a `checksums.txt` file.

Create a tagged release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow builds and uploads assets named like:

```text
codex-quota_0.1.0_linux_amd64.zip
codex-quota_0.1.0_linux_arm64.zip
codex-quota_0.1.0_darwin_amd64.zip
codex-quota_0.1.0_darwin_arm64.zip
codex-quota_0.1.0_windows_amd64.zip
checksums.txt
```

Each zip keeps the loadable plugin file at the archive root:

```text
codex-quota.so
codex-quota.dylib
codex-quota.dll
```

After the release is available, register the GitHub repository in the CPA plugin
store using the repository URL:

```text
https://github.com/router-for-me/cpa-plugin-quota
```

The plugin metadata returned at load time already declares `codex-quota`,
version, repository, and the Management API capability.
