# Architecture

This document describes how SwarmCD is structured internally, how the sync lifecycle works, and how the various components interact. It's intended for contributors, operators who want to understand what's happening under the hood, and anyone curious about the design decisions.

---

## Table of Contents

- [High-Level Overview](#high-level-overview)
- [Project Structure](#project-structure)
- [Component Breakdown](#component-breakdown)
  - [cmd/main.go — Entry Point](#cmdmaingo--entry-point)
  - [util — Configuration, Logging, and SOPS](#util--configuration-logging-and-sops)
  - [swarmcd — Core Engine](#swarmcd--core-engine)
  - [web — HTTP API and UI](#web--http-api-and-ui)
  - [ui — Frontend](#ui--frontend)
- [The Sync Lifecycle](#the-sync-lifecycle)
  - [1. Initialization](#1-initialization)
  - [2. The Main Loop](#2-the-main-loop)
  - [3. Updating a Single Stack](#3-updating-a-single-stack)
  - [4. Repository Locking](#4-repository-locking)
- [Data Flow Diagram](#data-flow-diagram)
- [Key Design Decisions](#key-design-decisions)
  - [Polling vs. Webhooks](#polling-vs-webhooks)
  - [In-Place Decryption](#in-place-decryption)
  - [Content-Hash Rotation](#content-hash-rotation)
  - [Docker CLI Library](#docker-cli-library)
  - [Concurrent Stack Updates with Repo Locking](#concurrent-stack-updates-with-repo-locking)
- [Configuration Loading](#configuration-loading)
- [Docker Image Build](#docker-image-build)
- [Technology Stack](#technology-stack)

---

## High-Level Overview

SwarmCD is a single Go binary that runs as a Docker Swarm service. It performs three roles simultaneously:

1. **GitOps controller** — clones Git repositories, polls them for changes on a configurable interval, and deploys Docker Swarm stacks using `docker stack deploy`.
2. **Web API server** — serves a `GET /stacks` JSON endpoint that returns the current status of all managed stacks.
3. **Web UI host** — serves a static React single-page application that visualizes stack status by consuming the API.

```
┌─────────────────────────────────────────────────┐
│                   SwarmCD Process                │
│                                                  │
│  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  GitOps Loop │  │   Gin HTTP Server        │  │
│  │              │  │                          │  │
│  │  • Pull repos│  │  GET /stacks  → JSON API │  │
│  │  • Render    │  │  GET /ui      → React UI │  │
│  │  • Decrypt   │  │  GET /assets  → Static   │  │
│  │  • Deploy    │  │                          │  │
│  └──────┬───────┘  └──────────────────────────┘  │
│         │                                        │
│         ▼                                        │
│  Docker Swarm API (via Docker socket or          │
│  DOCKER_HOST)                                    │
└─────────────────────────────────────────────────┘
```

The GitOps loop and the HTTP server run concurrently in separate goroutines.

---

## Project Structure

```
swarm-cd-arm/
├── cmd/
│   └── main.go              # Application entry point
├── swarmcd/
│   ├── init.go              # Initialization: repos, stacks, Docker CLI
│   ├── init_test.go
│   ├── repo.go              # Git repository operations (clone, pull)
│   ├── stack.go             # Stack operations (read, render, decrypt, deploy)
│   ├── stack_test.go
│   └── swarmcd.go           # Main sync loop and stack status
├── util/
│   ├── config.go            # Configuration loading (Viper)
│   ├── config_test.go
│   ├── log.go               # Structured logger setup (slog)
│   ├── log_test.go
│   ├── sops.go              # SOPS decryption wrapper
│   └── sops_test.go
├── web/
│   ├── routes.go            # Gin router setup and static file serving
│   ├── controllers.go       # API endpoint handlers
│   └── controllers_test.go
├── ui/                      # React frontend (Vite + Chakra UI)
│   ├── src/
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   ├── components/
│   │   │   ├── HeaderBar.tsx
│   │   │   ├── StatusCard.tsx
│   │   │   ├── StatusCardList.tsx
│   │   │   └── ColorToggleButton.tsx
│   │   └── hooks/
│   │       └── useFetchStatuses.ts
│   └── ...
├── docs/                    # Documentation
├── assets/                  # Screenshots
├── Dockerfile               # Multi-stage build
├── entrypoint.sh            # GPG key import at container startup
├── go.mod / go.sum          # Go module dependencies
└── .github/workflows/       # CI/CD pipeline
```

---

## Component Breakdown

### cmd/main.go — Entry Point

The entry point is intentionally minimal. It does three things:

1. **Loads configuration** — calls `util.LoadConfigs()` to read `config.yaml`, `repos.yaml`, and `stacks.yaml`.
2. **Initializes the engine** — calls `swarmcd.Init()` to clone repos, validate stack definitions, and set up the Docker CLI client.
3. **Starts both loops** — launches the GitOps sync loop (`swarmcd.Run()`) in a background goroutine and starts the HTTP server (`web.RunServer()`) on the main goroutine.

If either initialization step fails, the process exits immediately with an error message.

### util — Configuration, Logging, and SOPS

This package provides shared infrastructure used by the rest of the application.

**config.go** — Uses [Viper](https://github.com/spf13/viper) to read configuration from YAML files. The loading strategy is:

1. Read `config.yaml` (optional) and apply defaults for all global settings.
2. If `config.yaml` defines `repos:`, use those. Otherwise, read `repos.yaml`.
3. If `config.yaml` defines `stacks:`, use those. Otherwise, read `stacks.yaml`.

Key structs:

| Struct | Purpose |
|---|---|
| `Config` | Top-level configuration holding all global settings, repo configs, and stack configs |
| `RepoConfig` | A single Git repository definition (URL, credentials) |
| `StackConfig` | A single stack definition (repo, branch, compose path, SOPS files, values file, env files) |
| `EnvFileConfig` | A single env file reference (path, optional repo/branch override) |

The global `util.Configs` variable holds the loaded configuration and is accessed throughout the application.

**log.go** — Sets up Go's standard `slog` structured logger. The log level is read from the `LOG_LEVEL` environment variable (default: `info`). All components share the `util.Logger` instance.

**sops.go** — Wraps the SOPS `decrypt` library. The `DecryptFile()` function reads an encrypted file, determines its format from the file extension, decrypts it in place, and writes the plaintext back to the same path. Supported formats: YAML, JSON, INI, dotenv, and binary (for everything else).

### swarmcd — Core Engine

This is the heart of SwarmCD. It contains all GitOps logic.

**init.go** — The `Init()` function runs at startup and performs three steps:

1. **`initRepos()`** — For each repo defined in configuration, clones it to the local filesystem (or opens it if already cloned). Sets up HTTP Basic authentication if credentials are provided. Each repo gets a `sync.Mutex` for concurrency control.

2. **`initStacks()`** — For each stack defined in configuration, validates that the referenced repo exists, resolves env file references (which may point to different repos/branches), and creates the internal `swarmStack` struct. Initializes the `stackStatus` map that tracks each stack's current state.

3. **`initDockerCli()`** — Creates a Docker CLI client using the `docker/cli` library. This client is used to run `docker stack deploy` commands programmatically. Output streams are redirected to `/dev/null` since errors are returned as Go error values.

**repo.go** — Defines the `stackRepo` struct, which wraps a `go-git` repository. Key operations:

- **`newStackRepo()`** — Clones the repo (or opens an existing clone). Handles the case where the repo was previously cloned and still exists on disk.
- **`pullChanges(branch)`** — Checks out the remote tracking branch (`refs/remotes/origin/<branch>`), pulls the latest changes, and returns the short (8-character) commit hash of HEAD. The checkout uses `Force: true` to discard any local modifications (like in-place SOPS decryption from the previous cycle).

Each `stackRepo` has a `sync.Mutex` (`lock`) because multiple stacks may reference the same repo, and Git operations are not safe to run concurrently on the same working directory.

**stack.go** — Defines the `swarmStack` struct and the full update pipeline. This is where most of the processing logic lives. Key operations:

- **`updateStack()`** — Orchestrates the full update pipeline (see [The Sync Lifecycle](#the-sync-lifecycle)).
- **`readStack()`** — Reads the raw Compose file from the cloned repo.
- **`loadEnvFiles()`** — Reads `.env` files (potentially from multiple repos), parses them, and merges them into a single key-value map.
- **`substituteEnvVars()`** — Replaces `${VAR}`, `$VAR`, `${VAR:-default}`, `${VAR-default}`, and `$$` references in the Compose file content.
- **`renderComposeTemplate()`** — Parses the Compose file as a Go `text/template`, reads the values YAML file, and executes the template with values available under `.Values`.
- **`parseStackString()`** — Unmarshals the (potentially rendered) Compose YAML into a `map[string]any`.
- **`decryptSopsFiles()`** — Decrypts SOPS-encrypted files, either from an explicit list or via auto-discovery.
- **`discoverSecrets()`** — Inspects the `secrets:` section of the Compose map and collects file paths for non-external secrets.
- **`rotateConfigsAndSecrets()`** — For each config/secret defined in the Compose file, reads the referenced file, computes an MD5 hash, and sets the `name` field to `<stack>-<object>-<hash>`. This forces Docker Swarm to treat modified files as new objects.
- **`writeStack()`** — Marshals the (potentially modified) Compose map back to YAML and writes it to disk.
- **`deployStack()`** — Runs `docker stack deploy --detach --with-registry-auth -c <compose-file> <stack-name>` using the Docker CLI library.

**swarmcd.go** — Contains the main loop and the global stack status map:

- **`Run()`** — Infinite loop that iterates over all stacks, launches a goroutine for each, waits for all to finish, then sleeps for `update_interval` seconds.
- **`updateStackThread()`** — Acquires the repo lock, calls `updateStack()`, and updates the `stackStatus` map with the result (revision and/or error).
- **`GetStackStatus()`** — Returns the status map, consumed by the web API.

### web — HTTP API and UI

**routes.go** — Sets up the Gin router with structured logging middleware (`slog-gin`). Defines routes:

| Route | Handler | Description |
|---|---|---|
| `GET /` | redirect | Redirects to `/ui` |
| `GET /ui` | static file | Serves `ui/index.html` |
| `GET /assets/*` | static directory | Serves the Vite-built frontend assets |
| `GET /stacks` | `getStacks()` | Returns JSON array of stack statuses |

**controllers.go** — The `getStacks()` handler reads from `swarmcd.GetStackStatus()`, sorts the stacks alphabetically by name, and returns a JSON array. Each element contains `Name`, `Error`, `RepoURL`, and `Revision`.

### ui — Frontend

A React SPA built with [Vite](https://vitejs.dev/) and [Chakra UI](https://chakra-ui.com/). It's intentionally simple:

- **`App.tsx`** — Root component. Uses the `useFetchStatuses` hook to poll the `/stacks` API. Renders a search bar and a list of status cards.
- **`useFetchStatuses`** — Custom hook that periodically fetches `/stacks` and returns the data (or an error).
- **`StatusCardList`** — Filters and renders `StatusCard` components based on a search query.
- **`StatusCard`** — Displays a single stack's name, revision, repo URL, and error state.
- **`HeaderBar`** — Top bar with search input, GitHub link, and dark/light mode toggle.
- **`ColorToggleButton`** — Dark/light theme toggle.

The frontend is built at Docker image build time and served as static files by the Go backend.

---

## The Sync Lifecycle

### 1. Initialization

At startup (before the first sync cycle):

```
LoadConfigs()
  ├── Read config.yaml (defaults applied)
  ├── Read repos.yaml (if not inline)
  └── Read stacks.yaml (if not inline)

Init()
  ├── initRepos()
  │   └── For each repo: git clone (or open existing)
  ├── initStacks()
  │   └── For each stack: validate repo ref, resolve env files, create swarmStack
  └── initDockerCli()
      └── Create Docker client (connects to socket or DOCKER_HOST)
```

### 2. The Main Loop

After initialization, two goroutines run concurrently:

- **Goroutine 1 (background):** `swarmcd.Run()` — the sync loop
- **Goroutine 2 (main):** `web.RunServer()` — the HTTP server

The sync loop repeats indefinitely:

```
loop {
    for each stack (in parallel goroutines):
        acquire repo lock
        updateStack()
        update stackStatus map
        release repo lock
    
    wait(update_interval seconds)
}
```

### 3. Updating a Single Stack

Each stack update follows this pipeline:

```
updateStack()
│
├── 1. pullChanges(branch)
│      • git checkout refs/remotes/origin/<branch> --force
│      • git pull origin <branch>
│      • return short commit hash
│
├── 2. readStack()
│      • Read compose file from disk (raw bytes)
│
├── 3. loadEnvFiles() [if env_files configured]
│      • For cross-repo env files: lock → pull → unlock that repo
│      • Parse each .env file into key-value pairs
│      • Merge maps (later files override earlier ones)
│
├── 4. substituteEnvVars() [if env map is non-empty]
│      • Replace ${VAR}, $VAR, ${VAR:-default}, etc.
│
├── 5. renderComposeTemplate() [if values_file configured]
│      • Read values.yaml → unmarshal to map
│      • Parse compose content as Go template
│      • Execute template with .Values
│
├── 6. parseStackString()
│      • Unmarshal YAML into map[string]any
│
├── 7. decryptSopsFiles() [if sops configured]
│      │
│      ├── [explicit mode] Decrypt each file in sops_files list
│      │
│      └── [discovery mode] Inspect compose secrets: section
│          • For each non-external secret with a file: field
│          • Resolve path relative to compose file directory
│          • Decrypt the file in place
│
├── 8. rotateConfigsAndSecrets() [if auto_rotate enabled]
│      • For each config/secret in the compose map:
│        • Read the referenced file
│        • Compute MD5 hash of contents
│        • Set name to <stack>-<object>-<hash[:8]>
│
├── 9. writeStack()
│      • Marshal modified compose map back to YAML
│      • Write to disk (overwriting the original)
│
└── 10. deployStack()
       • docker stack deploy --detach --with-registry-auth \
         -c <compose-path> <stack-name>
```

Steps 3–8 are conditional — they only execute if the corresponding feature is configured for the stack. The pipeline always runs steps 1, 2, 6, 9, and 10.

### 4. Repository Locking

Multiple stacks can reference the same Git repository. Since Git operations (checkout, pull) modify the working directory, concurrent access would cause corruption. SwarmCD handles this with a per-repo `sync.Mutex`:

- Before updating a stack, the sync loop acquires the stack's repo lock.
- The lock is held for the entire update pipeline (pull → read → render → decrypt → rotate → write → deploy).
- After the update completes (successfully or with an error), the lock is released.

This means stacks sharing a repo are updated **sequentially**, while stacks using different repos are updated **in parallel**.

Cross-repo env files add a wrinkle: if a stack's env file references a different repo, that repo's lock is briefly acquired just for the pull and file-read operations, then released. This prevents deadlocks while ensuring safe concurrent access.

---

## Data Flow Diagram

```
                    ┌──────────────┐
                    │  repos.yaml  │
                    │  stacks.yaml │
                    │  config.yaml │
                    └──────┬───────┘
                           │ Load at startup
                           ▼
                    ┌──────────────┐
                    │  util.Config │ (in-memory config)
                    └──────┬───────┘
                           │
              ┌────────────┼─────────────┐
              ▼            ▼             ▼
       ┌────────────┐ ┌────────┐  ┌───────────┐
       │ stackRepo  │ │ stack  │  │ DockerCli │
       │ (per repo) │ │ (each) │  │ (shared)  │
       └─────┬──────┘ └───┬────┘  └─────┬─────┘
             │             │             │
             │ git pull    │ read/render │ docker stack deploy
             ▼             ▼             ▼
       ┌──────────┐  ┌──────────┐  ┌──────────────┐
       │  Local   │  │ Compose  │  │ Docker Swarm │
       │  Clone   │──│  File    │──│    API       │
       └──────────┘  └──────────┘  └──────────────┘
                           │
                    ┌──────┴───────┐
                    │ stackStatus  │ (in-memory map)
                    │  per stack:  │
                    │  • Revision  │
                    │  • Error     │
                    │  • RepoURL   │
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  GET /stacks │ (JSON API)
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   React UI   │
                    └──────────────┘
```

---

## Key Design Decisions

### Polling vs. Webhooks

SwarmCD uses a **polling** model rather than webhooks. On every sync cycle, it pulls every repo regardless of whether changes occurred. This design choice has trade-offs:

| | Polling | Webhooks |
|---|---|---|
| **Simplicity** | No incoming network configuration needed | Requires public endpoint, secret management |
| **Reliability** | Self-healing — missed changes are caught on the next cycle | Missed webhooks require retry logic |
| **Latency** | Bounded by `update_interval` (default 120s) | Near-instant |
| **Efficiency** | Pulls even when nothing changed | Only triggers on actual changes |

For the typical Docker Swarm use case (small-to-medium deployments), polling is simple, reliable, and good enough.

### In-Place Decryption

SOPS files are decrypted **in place** — the encrypted file on disk is overwritten with its plaintext contents. This works because:

1. The next sync cycle starts with `git pull --force`, which restores the encrypted version from the remote.
2. The `git checkout --force` discards any local modifications before pulling.

This avoids the complexity of managing temporary files or separate decrypted copies.

### Content-Hash Rotation

Docker Swarm configs and secrets are **immutable** — once created, their contents cannot be changed. To work around this, SwarmCD appends an MD5 hash of the file contents to the object name (e.g., `mystack-dbconfig-a1b2c3d4`). When the file changes, the hash changes, creating a new Swarm object, which triggers a service update.

This is controlled by the `auto_rotate` setting (default: `true`).

### Docker CLI Library

Instead of making raw Docker API calls, SwarmCD uses the `docker/cli` library and programmatically invokes `docker stack deploy`. This ensures 100% compatibility with the `docker stack deploy` command — including support for Compose file features like configs, secrets, networks, and deploy constraints — without having to reimplement the Compose-to-Swarm translation logic.

The trade-off is a dependency on the Docker CLI library, which is large and pulls in many transitive dependencies.

### Concurrent Stack Updates with Repo Locking

Stacks are updated concurrently using goroutines, but access to each repository's working directory is serialized with a mutex. This maximizes throughput when stacks use different repos while preventing corruption when they share one.

---

## Configuration Loading

The configuration loading order is important to understand:

```
1. config.yaml is read (if present)
   └── All fields get defaults if not specified:
       • update_interval: 120
       • repos_path: "repos"
       • auto_rotate: true
       • sops_secrets_discovery: false
       • address: "0.0.0.0:8080"

2. If config.yaml has "repos:" key → use those repos
   Else → read repos.yaml

3. If config.yaml has "stacks:" key → use those stacks
   Else → read stacks.yaml
```

This means you can use a single `config.yaml` for everything, or split configuration across three files. The separate-file approach is often preferred because it keeps concerns separated and makes it easier to update stack definitions without touching global settings.

---

## Docker Image Build

The Dockerfile uses a **three-stage build**:

```
Stage 1: Frontend Build (node:22-alpine)
  • npm install
  • npm run build (Vite production build)
  • Output: ui/dist/

Stage 2: Backend Build (golang:1.22-alpine)
  • go mod download
  • CGO_ENABLED=0 go build -o /swarm-cd ./cmd/
  • Output: /swarm-cd binary

Stage 3: Production Image (alpine:3.22)
  • Install ca-certificates and gnupg (for SOPS GPG support)
  • Copy binary from Stage 2
  • Copy frontend dist from Stage 1
  • Set GIN_MODE=release
  • Entrypoint: /entrypoint.sh (imports GPG keys if configured)
  • CMD: /app/swarm-cd
```

The `TARGETARCH` build argument enables multi-architecture builds (amd64 and arm64). The CI pipeline builds both architectures and creates a multi-arch manifest.

---

## Technology Stack

| Component | Technology | Purpose |
|---|---|---|
| **Language** | Go 1.22 | Backend, core engine |
| **Git operations** | [go-git](https://github.com/go-git/go-git) | Clone, checkout, pull without shelling out to `git` |
| **Docker operations** | [docker/cli](https://github.com/docker/cli) | Programmatic `docker stack deploy` |
| **HTTP framework** | [Gin](https://github.com/gin-gonic/gin) | REST API and static file serving |
| **Configuration** | [Viper](https://github.com/spf13/viper) | YAML config file loading with defaults |
| **Logging** | Go `slog` + [slog-gin](https://github.com/samber/slog-gin) | Structured logging |
| **SOPS** | [getsops/sops](https://github.com/getsops/sops) | Secret decryption (age, GPG, cloud KMS) |
| **YAML** | [goccy/go-yaml](https://github.com/goccy/go-yaml) | Compose file parsing and marshaling |
| **Frontend** | React 18 + TypeScript | Web UI |
| **UI framework** | [Chakra UI](https://chakra-ui.com/) | Component library |
| **Build tool** | [Vite](https://vitejs.dev/) | Frontend bundling |
| **Testing** | Go `testing` + [Vitest](https://vitest.dev/) | Backend and frontend tests |
| **CI/CD** | GitHub Actions + Semantic Release | Automated testing, building, and releasing |
| **Container** | Docker (multi-stage, multi-arch) | Packaging and distribution |
| **License** | GPL-3.0 | Open source |