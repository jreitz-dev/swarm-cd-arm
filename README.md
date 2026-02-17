# SwarmCD

**Declarative GitOps & Continuous Deployment for Docker Swarm**

SwarmCD watches your Git repositories for changes and automatically deploys and updates Docker Swarm stacks — no manual `docker stack deploy` required. Inspired by [ArgoCD](https://argo-cd.readthedocs.io/en/stable/), but purpose-built for Swarm.

![SwarmCD UI](assets/ui.png)

---

## Features

- **GitOps workflow** — define your stacks in Git; SwarmCD continuously reconciles the cluster to match.
- **Automatic sync** — polls repositories on a configurable interval and deploys changes hands-free, or disable polling to use webhooks exclusively.
- **Webhook support** — trigger stack deployments on-demand via HTTP webhook, ideal for CI/CD pipelines and instant deployments.
- **SOPS secret management** — decrypt [SOPS](https://github.com/getsops/sops)-encrypted files (age, GPG, AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault) before every deploy.
- **Automatic secret discovery** — optionally auto-detect `secrets:` entries in your Compose file so you don't have to list them manually.
- **Config & secret rotation** — automatically appends a content hash to Swarm config/secret names so services pick up changes without manual intervention.
- **Compose templating** — render your Compose files as Go templates with a separate `values.yaml`, or use `.env` file variable substitution with full docker-compose syntax (`${VAR}`, `${VAR:-default}`, `$$`).
- **Multi-repo support** — pull stacks, env files, and values from different repositories and branches.
- **Private repo auth** — authenticate with username/password or token files (ideal for Docker secrets).
- **Private registry auth** — pass Docker registry credentials via `~/.docker/config.json`.
- **Remote Docker hosts** — connect to remote or proxied Docker sockets via `DOCKER_HOST`.
- **Web UI** — lightweight React dashboard showing every stack's sync status, revision, and errors.
- **REST API** — `GET /stacks` returns JSON status for all managed stacks.
- **Multi-arch images** — published for `linux/amd64` and `linux/arm64`.

---

## Quick Start

### 1. Define your repositories

Create a `repos.yaml` file listing the Git repos that contain your stack Compose files:

```yaml
# repos.yaml
my-stacks:
  url: "https://github.com/you/my-stacks.git"
```

### 2. Define your stacks

Create a `stacks.yaml` file declaring which stacks to deploy:

```yaml
# stacks.yaml
nginx:
  repo: my-stacks
  branch: main
  compose_file: nginx/compose.yaml
```

### 3. Deploy SwarmCD

```yaml
# docker-compose.yaml
version: "3.7"
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    deploy:
      placement:
        constraints:
          - node.role == manager
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
```

```bash
docker stack deploy -c docker-compose.yaml swarm-cd
```

SwarmCD will clone your repo, deploy the `nginx` stack, and keep it in sync on every polling interval (default: **120 seconds**). Open `http://<manager-ip>:8080` to view the dashboard.

---

## Documentation

| Document | Description |
|---|---|
| [Getting Started](docs/getting-started.md) | Step-by-step first deployment walkthrough |
| [Configuration Reference](docs/configuration.md) | Every option for `config.yaml`, `repos.yaml`, and `stacks.yaml` |
| [Webhook](docs/webhook.md) | Trigger deployments on-demand via HTTP webhook |
| [Secrets Management](docs/secrets.md) | Using SOPS to encrypt and auto-decrypt secrets |
| [Templating & Env Files](docs/templating.md) | Go template rendering, values files, and `.env` substitution |
| [Deployment Patterns](docs/deployment.md) | Remote Docker sockets, private registries, socket proxies |
| [Architecture](docs/architecture.md) | How SwarmCD works under the hood |
| [REST API](docs/api.md) | API endpoint reference |
| [Contributing](docs/contributing.md) | Development setup, testing, and release process |

---

## Configuration at a Glance

SwarmCD reads up to three configuration files from its working directory (`/app` in the container):

| File | Required | Purpose |
|---|---|---|
| `repos.yaml` | Yes* | Git repository definitions |
| `stacks.yaml` | Yes* | Stack definitions (what to deploy) |
| `config.yaml` | No | Global settings (interval, address, feature flags) |

*Repos and stacks can alternatively be defined inside `config.yaml` under the `repos:` and `stacks:` keys.

### Minimal `config.yaml` example

```yaml
update_interval: 60        # seconds between sync cycles (default: 120)
polling_enabled: true       # set to false to disable polling and use webhooks only
address: 0.0.0.0:8080      # web UI / API listen address
auto_rotate: true           # hash-rename configs & secrets for rotation
sops_secrets_discovery: false
```

See the full [Configuration Reference](docs/configuration.md) for all options.

---

## Environment Variables

| Variable | Description |
|---|---|
| `DOCKER_HOST` | Connect to a remote Docker daemon (e.g. `tcp://socket-proxy:2375`) |
| `LOG_LEVEL` | Log verbosity: `debug`, `info` (default), `warn`, `error` |
| `SOPS_AGE_KEY_FILE` | Path to an [age](https://github.com/FiloSottile/age) private key for SOPS decryption |
| `SOPS_GPG_PRIVATE_KEY_FILE` | Path to a GPG private key file (imported automatically at startup) |
| `SOPS_GPG_PRIVATE_KEY` | GPG private key provided directly as an environment variable |
| `GIN_MODE` | Set to `release` in the published image; override to `debug` for verbose HTTP logging |

---

## Example: Full Production Setup

```yaml
version: "3.7"
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    deploy:
      placement:
        constraints:
          - node.role == manager
    environment:
      LOG_LEVEL: info
      SOPS_AGE_KEY_FILE: /secrets/age.key
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
      - ./config.yaml:/app/config.yaml:ro
    secrets:
      - source: age-key
        target: /secrets/age.key
      - source: docker-config
        target: /root/.docker/config.json
    ports:
      - "8080:8080"

secrets:
  age-key:
    file: ./age.key
  docker-config:
    file: ./docker-config.json
```

---

## License

SwarmCD is licensed under the [GNU General Public License v3.0](LICENSE).

---

## Acknowledgments

- Inspired by [ArgoCD](https://argo-cd.readthedocs.io/en/stable/)
- Secrets encryption powered by [SOPS](https://github.com/getsops/sops)
- Example stacks: [swarm-cd-example](https://github.com/m-adawi/swarm-cd-example)