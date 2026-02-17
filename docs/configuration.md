# Configuration Reference

SwarmCD reads up to three YAML configuration files from its working directory (`/app` inside the container). This document describes every available option.

| File | Required | Purpose |
|---|---|---|
| `repos.yaml` | Yes* | Git repository definitions |
| `stacks.yaml` | Yes* | Stack definitions — what to deploy and how |
| `config.yaml` | No | Global settings (polling interval, listen address, feature flags) |

> \* Repos and stacks can alternatively be defined inside `config.yaml` under the `repos:` and `stacks:` keys, making the separate files unnecessary.

---

## Table of Contents

- [config.yaml](#configyaml)
  - [update\_interval](#update_interval)
  - [polling\_enabled](#polling_enabled)
  - [repos\_path](#repos_path)
  - [auto\_rotate](#auto_rotate)
  - [sops\_secrets\_discovery](#sops_secrets_discovery)
  - [address](#address)
  - [repos](#repos)
  - [stacks](#stacks)
- [repos.yaml](#reposyaml)
  - [Repository entry fields](#repository-entry-fields)
  - [Authentication](#authentication)
- [stacks.yaml](#stacksyaml)
  - [Stack entry fields](#stack-entry-fields)
  - [env\_files](#env_files)
  - [sops\_files](#sops_files)
  - [sops\_secrets\_discovery (stack-level)](#sops_secrets_discovery-stack-level)
  - [values\_file](#values_file)
- [Environment Variables](#environment-variables)
- [Precedence and Merging Rules](#precedence-and-merging-rules)
- [Complete Examples](#complete-examples)

---

## config.yaml

Global settings that control SwarmCD's behavior. Every field is optional — sensible defaults are applied when the file is missing or a field is omitted.

### Full annotated example

```yaml
# config.yaml

# How often (in seconds) SwarmCD pulls repos and reconciles stacks.
# Default: 120
update_interval: 120

# Enable or disable automatic polling.
# When disabled, SwarmCD will only update stacks via webhook.
# Default: true
polling_enabled: true

# Local directory where SwarmCD clones Git repositories.
# Default: "repos"
repos_path: repos/

# When true, SwarmCD appends an MD5 content hash to the names of
# Swarm configs and secrets defined in your Compose files.
# This forces services to pick up changes without manual intervention.
# Default: true
auto_rotate: true

# When true, SwarmCD automatically discovers secret files declared
# in the `secrets:` section of every Compose file and decrypts them
# with SOPS before each deploy.
# When enabled globally, per-stack sops_secrets_discovery settings
# are ignored.
# Default: false
sops_secrets_discovery: false

# The address the web UI and REST API listen on.
# Default: "0.0.0.0:8080"
address: 0.0.0.0:8080

# You can inline repo definitions here instead of using a separate
# repos.yaml file. The format is identical to repos.yaml.
repos:
  # my-repo:
  #   url: "https://github.com/you/repo.git"

# You can inline stack definitions here instead of using a separate
# stacks.yaml file. The format is identical to stacks.yaml.
stacks:
  # my-stack:
  #   repo: my-repo
  #   branch: main
  #   compose_file: path/to/compose.yaml
```

### Property reference

#### update_interval

| | |
|---|---|
| **Type** | Integer (seconds) |
| **Default** | `120` |
| **Description** | Number of seconds SwarmCD waits between sync cycles. Each cycle pulls every repo and redeploys every stack. Lower values give faster convergence; higher values reduce Git and Docker API load. Ignored when `polling_enabled` is `false`. |

#### polling_enabled

| | |
|---|---|
| **Type** | Boolean |
| **Default** | `true` |
| **Description** | Controls whether SwarmCD automatically polls repositories on the `update_interval`. When set to `false`, the polling loop is completely disabled and stack updates will only occur via the webhook endpoint. This is useful when you want to rely exclusively on webhook-triggered deployments from CI/CD pipelines or Git hosting platforms. |

#### repos_path

| | |
|---|---|
| **Type** | String (directory path) |
| **Default** | `"repos"` |
| **Description** | Local filesystem path where SwarmCD clones repositories. Relative paths are resolved from the working directory (`/app`). Each repo is cloned into a subdirectory named after its key in `repos.yaml`. |

#### auto_rotate

| | |
|---|---|
| **Type** | Boolean |
| **Default** | `true` |
| **Description** | When enabled, SwarmCD reads every file referenced by top-level `configs:` and `secrets:` entries in your Compose file, computes an MD5 hash of the file contents, and renames the Swarm object to `<stack>-<name>-<hash>`. This ensures Docker Swarm treats modified config/secret files as new objects, triggering service updates automatically. |

#### sops_secrets_discovery

| | |
|---|---|
| **Type** | Boolean |
| **Default** | `false` |
| **Description** | Globally enable automatic SOPS secret discovery. When `true`, SwarmCD inspects the `secrets:` section of every stack's Compose file and decrypts each referenced file with SOPS before deploying. This overrides and ignores per-stack `sops_secrets_discovery` and `sops_files` settings. |

#### address

| | |
|---|---|
| **Type** | String (`host:port`) |
| **Default** | `"0.0.0.0:8080"` |
| **Description** | TCP address for the web UI and REST API. Set the host to `127.0.0.1` to restrict access to localhost, or use `0.0.0.0` to listen on all interfaces. |

#### repos

| | |
|---|---|
| **Type** | Map (same schema as `repos.yaml`) |
| **Default** | `null` |
| **Description** | Inline repo definitions. If present, `repos.yaml` is not read. See [repos.yaml](#reposyaml) for the schema. |

#### stacks

| | |
|---|---|
| **Type** | Map (same schema as `stacks.yaml`) |
| **Default** | `null` |
| **Description** | Inline stack definitions. If present, `stacks.yaml` is not read. See [stacks.yaml](#stacksyaml) for the schema. |

---

## repos.yaml

Defines the Git repositories that contain your stack Compose files and related assets. Each top-level key is a **repo name** — a unique identifier used to reference the repo from `stacks.yaml`.

### Full annotated example

```yaml
# repos.yaml

# Public repository — no authentication needed
my-public-repo:
  url: "https://github.com/you/public-repo.git"

# Private repository — authenticate with a personal access token
my-private-repo:
  url: "https://github.com/you/private-repo.git"
  username: my-username
  password: ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Private repository — token stored in a file (recommended for production)
my-secure-repo:
  url: "https://github.com/you/secure-repo.git"
  username: my-username
  password_file: /run/secrets/repo-token
```

### Repository entry fields

| Field | Type | Required | Description |
|---|---|---|---|
| `url` | String | **Yes** | Git clone URL. HTTPS URLs are supported. |
| `username` | String | No | Username for HTTP Basic authentication. Required if `password` or `password_file` is set. |
| `password` | String | No | Password or personal access token for authentication. Mutually exclusive with `password_file` — if both are set, `password` takes precedence. |
| `password_file` | String | No | Path to a file containing the password or token. Leading/trailing whitespace and newlines are trimmed. This is the recommended approach for production — mount the token as a Docker secret. |

### Authentication

For **public repositories**, only the `url` field is required. For **private repositories**, you must provide `username` and one of `password` or `password_file`.

**Using Docker secrets (recommended):**

```yaml
# docker-compose.yaml (SwarmCD's own deployment)
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    secrets:
      - source: repo-token
        target: /run/secrets/repo-token
    # ...

secrets:
  repo-token:
    file: ./repo-token.txt
```

```yaml
# repos.yaml
my-repo:
  url: "https://github.com/you/private-repo.git"
  username: my-username
  password_file: /run/secrets/repo-token
```

> **Security note:** Avoid putting raw passwords or tokens in `repos.yaml` for production deployments. Use `password_file` and Docker secrets instead.

---

## stacks.yaml

Defines the Docker Swarm stacks that SwarmCD should deploy and keep in sync. Each top-level key is a **stack name** — this becomes the stack name in `docker stack deploy <name>` and is what you'll see in `docker stack ls`.

### Full annotated example

```yaml
# stacks.yaml

# Minimal stack definition
nginx:
  repo: my-repo
  branch: main
  compose_file: nginx/compose.yaml

# Stack with SOPS-encrypted secrets
nginx-ssl:
  repo: my-repo
  branch: main
  compose_file: nginx-ssl/compose.yaml
  sops_files:
    - nginx-ssl/secrets/tls.crt
    - nginx-ssl/secrets/tls.key

# Stack with automatic secret discovery
webapp:
  repo: my-repo
  branch: production
  compose_file: webapp/compose.yaml
  sops_secrets_discovery: true

# Stack with Go template rendering
parameterized-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml.tmpl
  values_file: app/values.yaml

# Stack with .env file substitution
configured-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  env_files:
    # Env file from the same repo/branch as the stack
    - path: app/defaults.env
    # Env file from a different repo and branch
    - path: envs/prod/app.env
      repo: my-infra-repo
      branch: main
```

### Stack entry fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `repo` | String | **Yes** | — | Name of a repository defined in `repos.yaml` (or `config.yaml` `repos:`). |
| `branch` | String | **Yes** | — | Git branch to check out before each deploy. |
| `compose_file` | String | **Yes** | — | Path to the Docker Compose file, relative to the repository root. |
| `values_file` | String | No | `""` | Path to a YAML values file for Go template rendering. When set, the Compose file is parsed as a Go template. See [Templating](templating.md). |
| `env_files` | List | No | `[]` | Ordered list of `.env` files to load for `${VAR}` substitution. See [env\_files](#env_files). |
| `sops_files` | List of strings | No | `[]` | Paths to SOPS-encrypted files to decrypt before each deploy. Paths are relative to the repo root. See [Secrets Management](secrets.md). |
| `sops_secrets_discovery` | Boolean | No | `false` | Auto-discover secrets from the Compose file's `secrets:` section. When `true`, `sops_files` is ignored. See [sops\_secrets\_discovery (stack-level)](#sops_secrets_discovery-stack-level). |

### env_files

The `env_files` field is an ordered list of environment files to load before deploying the stack. Variables defined in later files override those in earlier files. Each entry supports the following fields:

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `path` | String | **Yes** | — | Path to the `.env` file, relative to the repo root. |
| `repo` | String | No | *(stack's repo)* | Name of a repo defined in `repos.yaml`. If omitted, defaults to the stack's own `repo`. |
| `branch` | String | No | *(stack's branch)* | Branch to check out in the env file's repo. If omitted, defaults to the stack's own `branch`. |

**Supported substitution syntax** (follows docker-compose conventions):

| Syntax | Behavior |
|---|---|
| `${VAR}` | Substitute with value from env map; empty string if unset |
| `$VAR` | Same as `${VAR}` |
| `${VAR:-default}` | Use `default` if `VAR` is unset **or empty** |
| `${VAR-default}` | Use `default` only if `VAR` is **unset** (empty string is kept) |
| `$$` | Literal `$` character |

**Env file format:**

```env
# Comments are supported
KEY=value
QUOTED_KEY="value with spaces"
SINGLE_QUOTED='literal value'

# "export" prefix is also supported
export EXPORTED_KEY=value
```

### sops_files

A list of file paths (relative to the repo root) that should be decrypted with [SOPS](https://github.com/getsops/sops) before each deploy. SwarmCD supports the following file formats:

| Extension | SOPS format |
|---|---|
| `.yaml`, `.yml` | yaml |
| `.json` | json |
| `.ini` | ini |
| `.env` | dotenv |
| *(anything else)* | binary |

See [Secrets Management](secrets.md) for full setup instructions including encryption key configuration.

### sops_secrets_discovery (stack-level)

When set to `true`, SwarmCD inspects the stack's Compose file for top-level `secrets:` entries with a `file:` field and automatically decrypts each referenced file with SOPS. External secrets (with `external: true`) are skipped.

**Precedence rules:**

1. If `sops_secrets_discovery` is `true` in `config.yaml` (global), it applies to **all** stacks and per-stack settings are ignored.
2. If `sops_secrets_discovery` is `true` on a specific stack, it overrides and ignores that stack's `sops_files` list.
3. If neither is set, only files explicitly listed in `sops_files` are decrypted.

### values_file

Path to a YAML file (relative to the repo root) whose contents are made available in the Compose file as Go template variables under `.Values`.

Example values file:

```yaml
# values.yaml
replicas: 3
image_tag: "v2.1.0"
domain: "example.com"
```

Example Compose template:

```yaml
services:
  web:
    image: myapp:{{ .Values.image_tag }}
    deploy:
      replicas: {{ .Values.replicas }}
```

See [Templating & Env Files](templating.md) for full details.

---

## Environment Variables

These environment variables affect SwarmCD's behavior at runtime. They are set on the SwarmCD container, not in config files.

| Variable | Description | Default |
|---|---|---|
| `DOCKER_HOST` | Docker daemon endpoint. Use this to connect to a remote host or socket proxy (e.g. `tcp://socket-proxy:2375`). | Unix socket (local) |
| `LOG_LEVEL` | Log verbosity. One of: `debug`, `info`, `warn`, `error`. | `info` |
| `GIN_MODE` | Gin web framework mode. Set to `release` in the published image. Override to `debug` for verbose HTTP request logging. | `release` |
| `SOPS_AGE_KEY_FILE` | Path to an [age](https://github.com/FiloSottile/age) private key file for SOPS decryption. | — |
| `SOPS_GPG_PRIVATE_KEY_FILE` | Path to a GPG private key file. SwarmCD's entrypoint script automatically imports this key into the GPG keyring at startup. | — |
| `SOPS_GPG_PRIVATE_KEY` | GPG private key provided directly as a string. Imported into the keyring at startup if `SOPS_GPG_PRIVATE_KEY_FILE` is not set. | — |

> **Note:** SOPS also supports AWS KMS, GCP KMS, Azure Key Vault, and HashiCorp Vault. For those backends, set the appropriate environment variables as described in the [SOPS documentation](https://github.com/getsops/sops#usage).

---

## Precedence and Merging Rules

### Config file loading order

1. `config.yaml` is read first (if it exists). All fields receive their default values if not specified.
2. If `config.yaml` contains a `repos:` key, `repos.yaml` is **not** read.
3. If `config.yaml` contains a `stacks:` key, `stacks.yaml` is **not** read.
4. If `repos:` or `stacks:` are absent from `config.yaml`, the corresponding standalone files are loaded.

### SOPS secret discovery precedence

| Global `sops_secrets_discovery` | Stack `sops_secrets_discovery` | Stack `sops_files` | Result |
|---|---|---|---|
| `true` | *(ignored)* | *(ignored)* | Auto-discover for all stacks |
| `false` | `true` | *(ignored)* | Auto-discover for this stack only |
| `false` | `false` | `[file1, file2]` | Decrypt listed files only |
| `false` | `false` | `[]` | No SOPS decryption |

---

## Complete Examples

### Single-file configuration (everything in config.yaml)

```yaml
# config.yaml
update_interval: 60
address: 0.0.0.0:9090
auto_rotate: true
sops_secrets_discovery: false

repos:
  my-stacks:
    url: "https://github.com/you/my-stacks.git"
  my-infra:
    url: "https://github.com/you/infra.git"
    username: deploy-bot
    password_file: /run/secrets/infra-token

stacks:
  frontend:
    repo: my-stacks
    branch: main
    compose_file: frontend/compose.yaml

  backend:
    repo: my-stacks
    branch: main
    compose_file: backend/compose.yaml
    values_file: backend/values-prod.yaml
    env_files:
      - path: backend/defaults.env
      - path: envs/prod/backend.env
        repo: my-infra
        branch: main

  monitoring:
    repo: my-infra
    branch: main
    compose_file: monitoring/compose.yaml
    sops_secrets_discovery: true
```

### Multi-file configuration (separate repos.yaml + stacks.yaml)

```yaml
# repos.yaml
app-repo:
  url: "https://github.com/you/app.git"

infra-repo:
  url: "https://github.com/you/infra.git"
  username: ci-bot
  password_file: /run/secrets/github-token
```

```yaml
# stacks.yaml
app:
  repo: app-repo
  branch: production
  compose_file: deploy/compose.yaml
  values_file: deploy/values.yaml
  sops_files:
    - deploy/secrets/db-password.enc.yaml
    - deploy/secrets/api-key.enc.yaml

monitoring:
  repo: infra-repo
  branch: main
  compose_file: monitoring/compose.yaml
  sops_secrets_discovery: true
  env_files:
    - path: monitoring/defaults.env
```

```yaml
# config.yaml
update_interval: 90
auto_rotate: true
address: 0.0.0.0:8080
```

### Webhook-only configuration (polling disabled)

```yaml
# repos.yaml
app-repo:
  url: "https://github.com/you/app.git"
  username: ci-bot
  password_file: /run/secrets/github-token
```

```yaml
# stacks.yaml
app:
  repo: app-repo
  branch: production
  compose_file: deploy/compose.yaml
```

```yaml
# config.yaml
polling_enabled: false
webhook_key_file: /run/secrets/webhook_key
address: 0.0.0.0:8080
auto_rotate: true
```

In this configuration, SwarmCD will only deploy when triggered via the webhook endpoint. The polling loop is completely disabled, and all deployments must be initiated by your CI/CD pipeline or Git hosting platform.
