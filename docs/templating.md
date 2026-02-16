# Templating & Env Files

SwarmCD provides two complementary mechanisms for parameterizing your Docker Compose files before deployment:

1. **Go template rendering** — treat the Compose file as a [Go template](https://pkg.go.dev/text/template) and inject values from a separate YAML file.
2. **`.env` file variable substitution** — load key-value pairs from one or more `.env` files and substitute `${VAR}` references in the Compose file using docker-compose syntax.

Both features are optional and can be used independently or together. When both are configured, environment variable substitution runs **first**, followed by Go template rendering.

---

## Table of Contents

- [Go Template Rendering](#go-template-rendering)
  - [How it works](#how-it-works)
  - [Setting up a values file](#setting-up-a-values-file)
  - [Template syntax](#template-syntax)
  - [Accessing values](#accessing-values)
  - [Template functions](#template-functions)
  - [Example: Parameterized stack](#example-parameterized-stack)
- [Environment Variable Substitution](#environment-variable-substitution)
  - [How it works](#how-it-works-1)
  - [Configuring env files](#configuring-env-files)
  - [Substitution syntax](#substitution-syntax)
  - [Env file format](#env-file-format)
  - [Cross-repo env files](#cross-repo-env-files)
  - [Example: Multi-environment deployment](#example-multi-environment-deployment)
- [Using Both Together](#using-both-together)
  - [Processing order](#processing-order)
  - [Example: Env files + Go templates](#example-env-files--go-templates)
- [Troubleshooting](#troubleshooting)

---

## Go Template Rendering

### How it works

When a stack has a `values_file` configured in `stacks.yaml`, SwarmCD treats the Compose file as a [Go `text/template`](https://pkg.go.dev/text/template). Before deploying, it:

1. Reads the values YAML file and parses it into a map.
2. Parses the Compose file as a Go template.
3. Executes the template, passing the values map under the key `.Values`.
4. Uses the rendered output as the final Compose file for `docker stack deploy`.

### Setting up a values file

Create a YAML file in your repository with the parameters you want to inject:

```yaml
# app/values.yaml
replicas: 3
image_tag: "v2.1.0"
domain: "example.com"
log_level: "warn"
resources:
  memory_limit: "512M"
  cpu_limit: "0.5"
```

Then reference it in your stack definition:

```yaml
# stacks.yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  values_file: app/values.yaml
```

Both `compose_file` and `values_file` are paths relative to the repository root.

### Template syntax

Go templates use `{{ }}` delimiters for actions. Here are the constructs you'll use most often:

| Syntax | Description |
|---|---|
| `{{ .Values.key }}` | Insert a value |
| `{{ .Values.nested.key }}` | Insert a nested value (via map access) |
| `{{ if .Values.flag }}...{{ end }}` | Conditional block |
| `{{ if .Values.flag }}...{{ else }}...{{ end }}` | Conditional with else |
| `{{ range .Values.list }}...{{ end }}` | Iterate over a list |
| `{{- ... -}}` | Trim surrounding whitespace |
| `{{ /* comment */ }}` | Template comment (not included in output) |

### Accessing values

All values from the YAML file are available under `.Values`. The keys in the values file become map keys:

**values.yaml:**

```yaml
image: myapp
tag: "v1.0"
ports:
  http: 8080
  https: 8443
```

**compose.yaml:**

```yaml
services:
  web:
    image: {{ .Values.image }}:{{ .Values.tag }}
    ports:
      - "{{ .Values.ports.http }}:8080"
      - "{{ .Values.ports.https }}:8443"
```

> **Note:** Nested maps in YAML are accessed with dot notation in the template. The values file is unmarshalled into `map[string]any`, so deeply nested structures work as expected.

### Template functions

SwarmCD uses Go's standard `text/template` package, which provides a set of [built-in functions](https://pkg.go.dev/text/template#hdr-Functions):

| Function | Example | Description |
|---|---|---|
| `and` | `{{ if and .Values.a .Values.b }}` | Logical AND |
| `or` | `{{ if or .Values.a .Values.b }}` | Logical OR |
| `not` | `{{ if not .Values.debug }}` | Logical NOT |
| `eq` | `{{ if eq .Values.env "prod" }}` | Equality check |
| `ne` | `{{ if ne .Values.env "dev" }}` | Not-equal check |
| `lt`, `le`, `gt`, `ge` | `{{ if gt .Values.replicas 1 }}` | Numeric comparisons |
| `len` | `{{ len .Values.hosts }}` | Length of a list or map |
| `index` | `{{ index .Values.hosts 0 }}` | Access element by index/key |
| `printf` | `{{ printf "%s:%s" .Values.image .Values.tag }}` | Formatted string |
| `html`, `js`, `urlquery` | `{{ urlquery .Values.path }}` | Escape helpers |

### Example: Parameterized stack

**Repository structure:**

```
app/
├── compose.yaml
└── values.yaml
```

**app/values.yaml:**

```yaml
image_tag: "v3.2.1"
replicas: 2
domain: "app.example.com"
debug: false
resources:
  memory: "256M"
  cpus: "0.25"
env:
  DATABASE_URL: "postgres://db:5432/app"
  REDIS_URL: "redis://cache:6379"
```

**app/compose.yaml:**

```yaml
version: "3.7"
services:
  web:
    image: myapp:{{ .Values.image_tag }}
    deploy:
      replicas: {{ .Values.replicas }}
      resources:
        limits:
          memory: {{ .Values.resources.memory }}
          cpus: "{{ .Values.resources.cpus }}"
    environment:
      {{- range $key, $value := .Values.env }}
      - {{ $key }}={{ $value }}
      {{- end }}
      {{ if .Values.debug -}}
      - DEBUG=true
      {{- end }}
    labels:
      - "traefik.http.routers.web.rule=Host(`{{ .Values.domain }}`)"
```

**stacks.yaml:**

```yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  values_file: app/values.yaml
```

**Rendered output:**

```yaml
version: "3.7"
services:
  web:
    image: myapp:v3.2.1
    deploy:
      replicas: 2
      resources:
        limits:
          memory: 256M
          cpus: "0.25"
    environment:
      - DATABASE_URL=postgres://db:5432/app
      - REDIS_URL=redis://cache:6379
    labels:
      - "traefik.http.routers.web.rule=Host(`app.example.com`)"
```

---

## Environment Variable Substitution

### How it works

When a stack has `env_files` configured, SwarmCD loads the specified `.env` files in order, building a key-value map. It then scans the Compose file content for `${VAR}` (and `$VAR`) references and replaces them with the corresponding values from the map.

This is similar to how `docker-compose` handles `.env` files and variable substitution.

### Configuring env files

Add an `env_files` list to your stack definition in `stacks.yaml`:

```yaml
# stacks.yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  env_files:
    - path: app/defaults.env
    - path: app/production.env
```

Files are loaded in the order they're listed. If the same variable is defined in multiple files, the **later file wins**.

Each entry in the `env_files` list supports these fields:

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `path` | String | **Yes** | — | Path to the `.env` file, relative to the repo root. |
| `repo` | String | No | *(stack's repo)* | Name of a repo defined in `repos.yaml`. Defaults to the stack's own repo. |
| `branch` | String | No | *(stack's branch)* | Branch to check out in the env file's repo. Defaults to the stack's own branch. |

### Substitution syntax

SwarmCD supports the standard docker-compose variable substitution syntax:

| Syntax | Behavior |
|---|---|
| `${VAR}` | Substitute with value from env map; empty string if the variable is not defined |
| `$VAR` | Same as `${VAR}` (bare variable reference) |
| `${VAR:-default}` | Use `default` if `VAR` is **unset or empty** |
| `${VAR-default}` | Use `default` only if `VAR` is **unset** (an empty string is kept as-is) |
| `$$` | Literal `$` character (escape sequence) |

**Examples:**

Given an env file containing:

```env
DB_HOST=postgres.example.com
DB_PORT=5432
DB_NAME=
```

The following substitutions would produce:

| Input | Output | Explanation |
|---|---|---|
| `${DB_HOST}` | `postgres.example.com` | Direct substitution |
| `${DB_PORT}` | `5432` | Direct substitution |
| `${DB_NAME}` | *(empty string)* | Variable is set but empty |
| `${DB_NAME:-mydb}` | `mydb` | Empty counts as unset for `:-` |
| `${DB_NAME-mydb}` | *(empty string)* | Variable is set (even though empty) for `-` |
| `${MISSING}` | *(empty string)* | Variable not defined |
| `${MISSING:-fallback}` | `fallback` | Not defined → use default |
| `${MISSING-fallback}` | `fallback` | Not defined → use default |
| `$$DB_HOST` | `$DB_HOST` | Escaped dollar sign |

### Env file format

SwarmCD's env file parser supports the following syntax:

```env
# Lines starting with # are comments
# Empty lines are ignored

# Simple key=value pairs
DATABASE_URL=postgres://localhost:5432/mydb
REDIS_URL=redis://localhost:6379

# Values can be quoted with double or single quotes
SECRET_KEY="my-secret-with-spaces"
API_TOKEN='literal-single-quoted-value'

# The "export" prefix is supported and stripped
export NODE_ENV=production
export LOG_LEVEL=info
```

**Rules:**

- Lines starting with `#` are treated as comments and ignored.
- Empty lines are ignored.
- Each non-empty, non-comment line must contain an `=` character.
- Leading and trailing whitespace on keys and values is trimmed.
- Matching quotes (single or double) wrapping the entire value are stripped.
- The `export ` prefix (with a trailing space) is stripped if present.

### Cross-repo env files

A common pattern is to keep environment-specific configuration in a separate "infrastructure" or "hosting" repository. SwarmCD supports this by letting you specify a different `repo` and `branch` for individual env file entries.

```yaml
# stacks.yaml
my-app:
  repo: app-repo
  branch: main
  compose_file: app/compose.yaml
  env_files:
    # Defaults from the app repo
    - path: app/defaults.env
    # Production overrides from the infra repo
    - path: envs/production/my-app.env
      repo: infra-repo
      branch: main
```

When an env file references a different repo, SwarmCD will:

1. Acquire a lock on that repo (to prevent concurrent access with other stacks).
2. Pull the latest changes for the specified branch.
3. Read the env file from the repo's working copy.
4. Release the lock.

The referenced repo must be defined in `repos.yaml`.

### Example: Multi-environment deployment

This example shows how to use env files to deploy the same stack with different configuration for staging and production.

**Repository structure (app-repo):**

```
app/
├── compose.yaml
└── defaults.env
```

**Repository structure (infra-repo):**

```
envs/
├── staging/
│   └── app.env
└── production/
    └── app.env
```

**app/defaults.env:**

```env
APP_PORT=8080
LOG_FORMAT=json
WORKERS=2
```

**envs/staging/app.env:**

```env
APP_DOMAIN=staging.example.com
DATABASE_URL=postgres://staging-db:5432/app
WORKERS=1
```

**envs/production/app.env:**

```env
APP_DOMAIN=app.example.com
DATABASE_URL=postgres://prod-db:5432/app
WORKERS=4
```

**app/compose.yaml:**

```yaml
version: "3.7"
services:
  web:
    image: myapp:latest
    environment:
      - APP_DOMAIN=${APP_DOMAIN}
      - DATABASE_URL=${DATABASE_URL}
      - LOG_FORMAT=${LOG_FORMAT}
      - WORKERS=${WORKERS:-2}
    ports:
      - "${APP_PORT}:8080"
```

**stacks.yaml:**

```yaml
app-staging:
  repo: app-repo
  branch: main
  compose_file: app/compose.yaml
  env_files:
    - path: app/defaults.env
    - path: envs/staging/app.env
      repo: infra-repo
      branch: main

app-production:
  repo: app-repo
  branch: main
  compose_file: app/compose.yaml
  env_files:
    - path: app/defaults.env
    - path: envs/production/app.env
      repo: infra-repo
      branch: main
```

For `app-staging`, the merged env map would be:

| Variable | Value | Source |
|---|---|---|
| `APP_PORT` | `8080` | defaults.env |
| `LOG_FORMAT` | `json` | defaults.env |
| `WORKERS` | `1` | staging/app.env (overrides default) |
| `APP_DOMAIN` | `staging.example.com` | staging/app.env |
| `DATABASE_URL` | `postgres://staging-db:5432/app` | staging/app.env |

---

## Using Both Together

### Processing order

When both `env_files` and `values_file` are configured on a stack, SwarmCD processes them in this order:

1. **Pull** the latest changes from the Git repo.
2. **Read** the raw Compose file.
3. **Load and substitute `.env` variables** — `${VAR}` references are replaced with values from the env map.
4. **Render the Go template** — the result from step 3 is parsed as a Go template and executed with the values file data.
5. **Parse** the rendered YAML.
6. **Decrypt** SOPS files (if configured).
7. **Rotate** configs and secrets (if enabled).
8. **Deploy** the stack.

This order means:

- Env variable substitution happens on the **raw file content** (before template parsing).
- Go template rendering happens on the **substituted content**.
- You can use `${VAR}` syntax inside Go template directives, but be mindful that the substitution happens first.

### Example: Env files + Go templates

This example uses env files for environment-specific secrets (like database URLs) and Go templates for structural parameterization (like replica counts and resource limits).

**app/defaults.env:**

```env
DATABASE_URL=postgres://localhost:5432/app
REDIS_URL=redis://localhost:6379
```

**app/values.yaml:**

```yaml
replicas: 3
image_tag: "v1.5.0"
resources:
  memory: "512M"
  cpus: "0.5"
```

**app/compose.yaml:**

```yaml
version: "3.7"
services:
  web:
    image: myapp:{{ .Values.image_tag }}
    deploy:
      replicas: {{ .Values.replicas }}
      resources:
        limits:
          memory: {{ .Values.resources.memory }}
          cpus: "{{ .Values.resources.cpus }}"
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - REDIS_URL=${REDIS_URL}
```

**stacks.yaml:**

```yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  values_file: app/values.yaml
  env_files:
    - path: app/defaults.env
```

**Processing:**

1. Env substitution replaces `${DATABASE_URL}` and `${REDIS_URL}` with their values.
2. Go template rendering replaces `{{ .Values.image_tag }}`, `{{ .Values.replicas }}`, etc.
3. The final rendered Compose file is deployed.

---

## Troubleshooting

### "could not parse ... compose file as a Go template"

Your Compose file has a syntax error in a Go template directive. Common causes:

- **Missing closing `}}`** — every `{{` must have a matching `}}`.
- **Typo in variable name** — `.Values.typo` will fail if `typo` doesn't exist in your values file. Go templates produce a `<no value>` output for missing keys by default, but accessing nested fields on a missing key causes an error.
- **Unquoted values in YAML** — if a template directive produces a value that contains special YAML characters (`:`, `#`, `{`, etc.), ensure the value is wrapped in quotes in the Compose file:

  ```yaml
  # Good — quoted
  image: "myapp:{{ .Values.tag }}"
  
  # Risky — may break YAML parsing depending on the value
  image: myapp:{{ .Values.tag }}
  ```

### "error rendering ... compose template"

The template parsed successfully but failed during execution. This usually means a value referenced in the template is missing or has the wrong type. Check that:

- The key exists in your values file.
- The type matches what the template expects (e.g., don't use `range` on a string).

### Env variables are not being substituted

- Verify that `env_files` is configured on the stack in `stacks.yaml`.
- Check that the `.env` file paths are correct and relative to the repo root.
- Ensure the env file uses proper `KEY=VALUE` syntax — lines without `=` are rejected.
- Set `LOG_LEVEL=debug` to see which env files SwarmCD loads and whether any errors occur during parsing.

### "invalid syntax at line N"

The env file parser found a line that is not empty, not a comment, and does not contain an `=` character. Fix the offending line in the env file.

### `$$` is being substituted instead of producing a literal `$`

Make sure you're using exactly `$$` (two dollar signs). The substitution engine will replace `$$` with a single `$` in the output. If you need a literal `$$` in the final output, use `$$$$`.

### Cross-repo env file not found

If an env file references a different repo via the `repo` field:

- Ensure the repo name matches a key in `repos.yaml`.
- Ensure the branch is correct (it defaults to the stack's branch, not necessarily `main`).
- Check that SwarmCD can successfully pull the referenced repo (authentication, network access, etc.).
- Set `LOG_LEVEL=debug` to see detailed pull and file-read logs.