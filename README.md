# Terraform Provider for Zeroword

Manages [Zeroword](https://docs.zeroword.smartcrab.ai/) resources through the Management API: Projects, Environments, Environment settings and Hosted Login themes, OAuth clients, social connections, Management API keys, custom domains, and webhooks. Resource documentation is in [`docs/`](docs/) and on the Terraform Registry.

```hcl
terraform {
  required_providers {
    zeroword = {
      source  = "smartcrabai/zeroword"
      version = "~> 0.1"
    }
  }
}

provider "zeroword" {
  # api_key defaults to ZEROWORD_API_KEY.
  base_url = "https://api.staging.zeroword.smartcrab.ai/v1" # omit for production
}
```

## Authentication

The provider uses one Workspace-scoped Management API key. An API key cannot create Workspaces or its first key, so create the key once in the Dashboard (Workspace menu → API keys) with the scopes for the resources you manage:

| Resources | Scopes |
| --- | --- |
| `zeroword_workspace` data source | `workspaces:read` |
| `zeroword_project` | `projects:read`, `projects:write` |
| `zeroword_environment`, `zeroword_environment_settings`, `zeroword_environment_theme` | `environments:read`, `environments:write` |
| `zeroword_client` | `clients:read`, `clients:write` |
| `zeroword_connection` | `connections:read`, `connections:write` |
| `zeroword_domain` | `domains:read`, `domains:write` |
| `zeroword_webhook` | `webhooks:read`, `webhooks:write` |
| `zeroword_api_key` | `api_keys:read`, `api_keys:write`, plus every scope the managed keys grant |

A key can only grant scopes it holds itself.

## Secrets and state

- `zeroword_api_key.secret`, `zeroword_client.client_secret`, and `zeroword_webhook.secret` are returned by Zeroword only at creation. The provider keeps them in state as sensitive values; after `terraform import` they are null.
- `zeroword_connection.provider_client_secret_wo` is write-only (Terraform 1.11+): it is sent to Zeroword and never stored in state. Change `provider_client_secret_wo_version` to send a rotated secret.
- Drift: every resource is re-read on refresh. A key revoked or expired outside Terraform, or a deleted resource, is planned for re-creation.

## Development

```sh
go test ./...                 # unit tests against an in-memory fake of the Management API
go generate ./...             # regenerate docs/ from the schemas and examples/
```

Acceptance tests run the same tests against a real Management API and create a `tfacc-*` Project in the Workspace:

```sh
TF_ACC=1 \
ZEROWORD_API_KEY=... \
ZEROWORD_BASE_URL=https://api.staging.zeroword.smartcrab.ai/v1 \
ZEROWORD_ACC_WORKSPACE_ID=wsp_... \
go test -v ./internal/provider
```

In GitHub Actions, run the **Acceptance** workflow manually. It reads the `ZEROWORD_ACC_API_KEY` secret and the `ZEROWORD_ACC_WORKSPACE_ID` variable (and `ZEROWORD_ACC_BASE_URL`, default staging).

## Releasing

Pushing a `v*` tag runs GoReleaser, which builds the binaries, signs the checksums with GPG, and publishes a GitHub release; the Terraform Registry imports it. One-time setup:

1. Create an RSA GPG key without expiry problems for the release (`gpg --full-generate-key`, RSA 4096). The Registry does not accept ECC keys.
2. Add the ASCII-armored public key (`gpg --armor --export <KEY_ID>`) in the Terraform Registry under the `smartcrabai` namespace (User settings → Signing keys).
3. Add the repository secrets `GPG_PRIVATE_KEY` (`gpg --armor --export-secret-keys <KEY_ID>`) and `PASSPHRASE`.
4. In the Terraform Registry, choose Publish → Provider and select `smartcrabai/terraform-provider-zeroword` (the Registry GitHub app needs access to the `smartcrabai` organization).
5. Push a tag, e.g. `git tag v0.1.0 && git push origin v0.1.0`.
