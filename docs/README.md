# SwarmCD Documentation

Welcome to the SwarmCD documentation. These guides cover everything from your first deployment to advanced production patterns.

## Guides

| Document | Description |
|---|---|
| [Getting Started](getting-started.md) | Step-by-step first deployment walkthrough |
| [Configuration Reference](configuration.md) | Every option for `config.yaml`, `repos.yaml`, and `stacks.yaml` |
| [Secrets Management](secrets.md) | Using SOPS to encrypt and auto-decrypt secrets |
| [Templating & Env Files](templating.md) | Go template rendering, values files, and `.env` variable substitution |
| [Deployment Patterns](deployment.md) | Remote Docker sockets, private registries, socket proxies |
| [Architecture](architecture.md) | How SwarmCD works under the hood |
| [REST API](api.md) | API endpoint reference |
| [Contributing](contributing.md) | Development setup, testing, and release process |

## Configuration File Reference (Quick Links)

These annotated example files show every available option with inline comments:

- [`config.yaml`](config.yaml) — global settings (update interval, listen address, feature flags)
- [`repos.yaml`](repos.yaml) — Git repository definitions (URLs, authentication)
- [`stacks.yaml`](stacks.yaml) — stack definitions (compose paths, branches, secrets, env files, templating)

## How It Works (TL;DR)

1. **You define** your Git repos in `repos.yaml` and your stacks in `stacks.yaml`.
2. **SwarmCD clones** the repos at startup and checks out the specified branches.
3. **On every sync cycle** (default: every 120 seconds), SwarmCD pulls the latest changes from each repo.
4. For each stack, it:
   - Reads the Compose file
   - Substitutes environment variables from `.env` files (if configured)
   - Renders Go templates with a values file (if configured)
   - Decrypts SOPS-encrypted secret files (if configured)
   - Appends content hashes to config/secret names for automatic rotation (if enabled)
   - Runs `docker stack deploy` to apply the result
5. **The web UI** at `http://<host>:8080` shows real-time status, current revision, and any errors for every managed stack.

## Common Tasks

### Deploy your first stack

→ [Getting Started](getting-started.md)

### Encrypt secrets with SOPS and have SwarmCD decrypt them automatically

→ [Secrets Management](secrets.md)

### Use Go templates or `.env` files to parameterize your Compose files

→ [Templating & Env Files](templating.md)

### Connect SwarmCD to a remote Docker host or use a socket proxy

→ [Deployment Patterns](deployment.md)

### Authenticate with private Git repos or container registries

→ [Configuration Reference — Repo Authentication](configuration.md#repos-yaml) and [Deployment Patterns — Private Registries](deployment.md#private-container-registries)

## Need Help?

- Check the [example repository](https://github.com/m-adawi/swarm-cd-example) for working stack definitions.
- Open an issue on [GitHub](https://github.com/m-adawi/swarm-cd/issues).