# Contributing

Thank you for your interest in contributing to SwarmCD! This guide covers everything you need to set up a development environment, run tests, understand the release process, and submit changes.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting the Source](#getting-the-source)
- [Project Layout](#project-layout)
- [Development Setup](#development-setup)
  - [Backend (Go)](#backend-go)
  - [Frontend (React)](#frontend-react)
- [Running Locally](#running-locally)
- [Testing](#testing)
  - [Go tests](#go-tests)
  - [Frontend tests](#frontend-tests)
  - [Running all tests](#running-all-tests)
- [Code Style](#code-style)
  - [Go](#go)
  - [TypeScript / React](#typescript--react)
- [Docker Build](#docker-build)
- [CI/CD Pipeline](#cicd-pipeline)
  - [What CI does](#what-ci-does)
  - [Triggering CI](#triggering-ci)
- [Release Process](#release-process)
  - [Semantic Release](#semantic-release)
  - [Commit message format](#commit-message-format)
  - [Multi-arch images](#multi-arch-images)
- [Submitting Changes](#submitting-changes)
  - [Opening a pull request](#opening-a-pull-request)
  - [Review process](#review-process)
- [Reporting Issues](#reporting-issues)
- [License](#license)

---

## Prerequisites

To work on SwarmCD, you'll need the following installed on your development machine:

| Tool | Version | Purpose |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.22+ | Backend development and testing |
| [Node.js](https://nodejs.org/) | 22+ | Frontend development and testing |
| [npm](https://www.npmjs.com/) | (bundled with Node) | Frontend dependency management |
| [Docker](https://docs.docker.com/get-docker/) | 20+ | Building and testing the container image |
| [Git](https://git-scm.com/) | 2.x | Version control |

Optional but recommended:

| Tool | Purpose |
|---|---|
| [Docker Swarm](https://docs.docker.com/engine/swarm/) | Testing end-to-end stack deployment locally |
| [SOPS](https://github.com/getsops/sops) | Testing secret encryption/decryption |
| [age](https://github.com/FiloSottile/age) | Generating encryption keys for SOPS testing |
| [jq](https://jqlang.github.io/jq/) | Inspecting JSON API responses |

---

## Getting the Source

Fork the repository on GitHub, then clone your fork:

```bash
git clone https://github.com/<your-username>/swarm-cd.git
cd swarm-cd
```

Add the upstream remote so you can pull in future changes:

```bash
git remote add upstream https://github.com/m-adawi/swarm-cd.git
```

---

## Project Layout

```
swarm-cd/
├── cmd/
│   └── main.go              # Application entry point
├── swarmcd/
│   ├── init.go              # Initialization (repos, stacks, Docker CLI)
│   ├── repo.go              # Git repository operations
│   ├── stack.go             # Stack update pipeline
│   ├── swarmcd.go           # Main sync loop
│   └── *_test.go            # Tests
├── util/
│   ├── config.go            # Configuration loading (Viper)
│   ├── log.go               # Structured logging (slog)
│   ├── sops.go              # SOPS decryption
│   └── *_test.go            # Tests
├── web/
│   ├── routes.go            # Gin router and static file serving
│   ├── controllers.go       # API handlers
│   └── *_test.go            # Tests
├── ui/                      # React frontend
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   └── hooks/
│   ├── tests/
│   ├── package.json
│   └── vite.config.ts
├── docs/                    # Documentation (you are here)
├── assets/                  # Screenshots
├── Dockerfile               # Multi-stage production build
├── entrypoint.sh            # Container entrypoint (GPG key import)
├── go.mod / go.sum
└── .github/workflows/       # CI/CD
```

See [Architecture](architecture.md) for a detailed breakdown of each component.

---

## Development Setup

### Backend (Go)

Install Go dependencies:

```bash
go mod download
```

Verify the build compiles:

```bash
go build -o swarm-cd ./cmd/
```

### Frontend (React)

Navigate to the `ui/` directory and install Node dependencies:

```bash
cd ui
npm install
```

Start the Vite development server (with hot reload):

```bash
npm run dev
```

This starts the frontend dev server at `http://localhost:5173`. Note that the dev server proxies API requests to the backend — you'll need the Go backend running simultaneously for the UI to function.

Build the production frontend:

```bash
npm run build
```

The built files are output to `ui/dist/`.

---

## Running Locally

To run SwarmCD locally for development, you need:

1. **A Docker Swarm cluster** — even a single-node swarm works:

   ```bash
   docker swarm init
   ```

2. **Configuration files** — create `repos.yaml` and `stacks.yaml` in the project root (they're `.gitignore`'d):

   ```yaml
   # repos.yaml
   example:
     url: "https://github.com/m-adawi/swarm-cd-example.git"
   ```

   ```yaml
   # stacks.yaml
   nginx:
     repo: example
     branch: main
     compose_file: nginx/compose.yaml
   ```

3. **Build the frontend** (the Go server expects it at `ui/dist/`):

   ```bash
   cd ui && npm run build && cd ..
   ```

4. **Run the backend**:

   ```bash
   go run ./cmd/
   ```

   Or with debug logging:

   ```bash
   LOG_LEVEL=debug go run ./cmd/
   ```

5. **Open the UI** at `http://localhost:8080`.

### Using the Vite dev server alongside the Go backend

For frontend development with hot reload, you can run both servers simultaneously:

- **Terminal 1:** `go run ./cmd/` (backend on port 8080)
- **Terminal 2:** `cd ui && npm run dev` (frontend dev server on port 5173)

The Vite dev server is configured to proxy `/stacks` requests to the Go backend. Access the UI via `http://localhost:5173` for hot-reload during frontend development.

---

## Testing

### Go tests

Run all Go tests from the project root:

```bash
go test -v ./...
```

Run tests for a specific package:

```bash
go test -v ./swarmcd/
go test -v ./util/
go test -v ./web/
```

Run a specific test function:

```bash
go test -v -run TestFunctionName ./package/
```

### Frontend tests

Run frontend tests from the `ui/` directory:

```bash
cd ui
npm run test
```

This runs [Vitest](https://vitest.dev/) with [happy-dom](https://github.com/nicedoc/happy-dom) as the DOM implementation and [@testing-library/react](https://testing-library.com/docs/react-testing-library/intro/) for component testing.

### Running all tests

To run both Go and frontend tests (as CI does):

```bash
go test -v ./...
cd ui && npm ci && npm run test
```

---

## Code Style

### Go

- Follow standard Go conventions and idioms.
- Use `gofmt` to format code (most editors do this automatically).
- Use meaningful variable and function names.
- Return errors rather than using `panic`.
- Use structured logging via `util.Logger` (Go's `slog` package).
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.

### TypeScript / React

- The project uses [ESLint](https://eslint.org/) and [Prettier](https://prettier.io/) for linting and formatting.
- Run the linter:

  ```bash
  cd ui
  npm run lint
  ```

- Prettier configuration is in `ui/.prettierrc.cjs`.
- ESLint configuration is in `ui/.eslintrc.cjs`.
- The `npm run build` command runs the linter automatically via the `prebuild` script. If linting fails, the build fails.

---

## Docker Build

Build the Docker image locally:

```bash
docker build -t swarm-cd:dev .
```

The Dockerfile uses a three-stage build:

1. **Stage 1 (Node):** Builds the frontend (`npm install` + `npm run build`)
2. **Stage 2 (Go):** Builds the backend binary (`go build`)
3. **Stage 3 (Alpine):** Copies the binary and frontend assets into a minimal production image

To build for a specific architecture:

```bash
# For arm64
docker build --build-arg TARGETARCH=arm64 -t swarm-cd:dev-arm64 .

# For amd64
docker build --build-arg TARGETARCH=amd64 -t swarm-cd:dev-amd64 .
```

Test the built image:

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v ./repos.yaml:/app/repos.yaml:ro \
  -v ./stacks.yaml:/app/stacks.yaml:ro \
  -p 8080:8080 \
  swarm-cd:dev
```

---

## CI/CD Pipeline

The CI/CD pipeline is defined in `.github/workflows/docker-ci.yaml`.

### What CI does

On every push that touches relevant files (Go source, `go.mod`, `go.sum`, `ui/**`, `Dockerfile`):

1. **Test Go** — runs `go test -v ./...` on Ubuntu with Go 1.22.
2. **Test Node** — runs `npm ci` and `npm run test` in the `ui/` directory with Node 22.
3. **Build amd64** — builds the Docker image for `linux/amd64` (runs after tests pass).
4. **Build arm64** — builds the Docker image for `linux/arm64` on an ARM runner (runs after tests pass).

On pushes to `main` only (after tests and builds pass):

5. **Semantic Release** — determines if a new version should be released based on commit messages.
6. **Push Multi-arch Manifest** — if a new version was released, creates and pushes a multi-arch Docker manifest to both GitHub Container Registry (`ghcr.io`) and Docker Hub.
7. **Update Docker Hub Description** — syncs the README to the Docker Hub repository description.

### Triggering CI

CI runs automatically on:

- **Every push** that modifies Go source files, Go module files, frontend files, or the Dockerfile.
- **Manual trigger** via `workflow_dispatch` in the GitHub Actions UI.

CI does **not** run if a push only modifies documentation, configuration references, or other non-code files.

---

## Release Process

### Semantic Release

SwarmCD uses [Semantic Release](https://semantic-release.gitbook.io/) to automate version management and changelog generation. Releases are determined entirely by commit messages — there is no manual version bumping.

The configuration is in `.releaserc.yaml`:

- Releases are created from the `main` branch only.
- Commit messages are analyzed using the [Conventional Commits](https://www.conventionalcommits.org/) standard.
- GitHub releases are created automatically with generated release notes.

### Commit message format

SwarmCD follows the [Conventional Commits](https://www.conventionalcommits.org/) specification. The commit message format determines whether a release is triggered and what version bump occurs:

| Prefix | Version Bump | Example |
|---|---|---|
| `fix:` | Patch (1.0.0 → 1.0.1) | `fix: handle empty sops_files list gracefully` |
| `feat:` | Minor (1.0.0 → 1.1.0) | `feat: add support for env file substitution` |
| `feat!:` or `BREAKING CHANGE:` in footer | Major (1.0.0 → 2.0.0) | `feat!: rename config_file to compose_file in stacks.yaml` |
| `docs:` | No release | `docs: update getting started guide` |
| `chore:` | No release | `chore: update dependencies` |
| `ci:` | No release | `ci: add arm64 build job` |
| `refactor:` | No release | `refactor: simplify stack update pipeline` |
| `test:` | No release | `test: add unit tests for env file parsing` |

**Examples of good commit messages:**

```
feat: add Go template rendering for compose files

Compose files can now be treated as Go templates when a values_file
is specified. Values are accessible under .Values in the template.

Closes #42
```

```
fix: trim whitespace from password_file contents

Leading and trailing whitespace (including newlines) in password
files was causing authentication failures with some Git providers.
```

```
feat!: change env_files format to support cross-repo references

BREAKING CHANGE: The env_files field in stacks.yaml now accepts a
list of objects with path, repo, and branch fields instead of a
plain list of strings.
```

### Multi-arch images

The CI pipeline builds Docker images for both `linux/amd64` and `linux/arm64`. Each architecture is built on its native runner (no emulation), and the results are combined into a multi-arch manifest that's pushed to both registries:

- `ghcr.io/m-adawi/swarm-cd:latest`
- `ghcr.io/m-adawi/swarm-cd:<version>`
- `<dockerhub-username>/swarm-cd:latest`
- `<dockerhub-username>/swarm-cd:<version>`

Users pulling the image will automatically get the correct architecture for their platform.

---

## Submitting Changes

### Opening a pull request

1. **Create a feature branch** from your fork's `main`:

   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make your changes.** Follow the code style guidelines and commit message format described above.

3. **Run tests locally** to make sure everything passes:

   ```bash
   go test -v ./...
   cd ui && npm run test && cd ..
   ```

4. **Push to your fork:**

   ```bash
   git push origin feat/my-feature
   ```

5. **Open a pull request** against the `main` branch of the upstream repository.

6. **Describe your changes** in the PR description:
   - What problem does this solve?
   - How does it work?
   - Are there any breaking changes?
   - How was this tested?

### Review process

- All pull requests are reviewed before merging.
- CI must pass (Go tests, frontend tests, Docker build) before a PR can be merged.
- Please respond to review feedback promptly. If a review request goes stale, the PR may be closed.
- Once approved, a maintainer will merge the PR. If the commit messages follow the Conventional Commits format, a release will be triggered automatically if warranted.

---

## Reporting Issues

If you encounter a bug, have a feature request, or need help:

1. **Search existing issues** on [GitHub](https://github.com/m-adawi/swarm-cd/issues) to see if it's already been reported.
2. **Open a new issue** with a clear title and description. For bugs, include:
   - SwarmCD version (or commit hash)
   - Your `config.yaml`, `repos.yaml`, and `stacks.yaml` (redact sensitive values)
   - Relevant log output (with `LOG_LEVEL=debug` if possible)
   - Steps to reproduce the issue
   - Expected vs. actual behavior

---

## License

SwarmCD is licensed under the [GNU General Public License v3.0](../LICENSE). By contributing to this project, you agree that your contributions will be licensed under the same license.