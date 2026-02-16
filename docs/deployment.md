# Deployment Patterns

This guide covers advanced deployment configurations for SwarmCD, including remote Docker sockets, socket proxies, private container registries, and production hardening tips.

---

## Table of Contents

- [Basic Deployment](#basic-deployment)
- [Remote Docker Sockets](#remote-docker-sockets)
  - [Using DOCKER\_HOST](#using-docker_host)
  - [Using a Docker Socket Proxy](#using-a-docker-socket-proxy)
  - [Socket proxy permissions](#socket-proxy-permissions)
- [Private Container Registries](#private-container-registries)
  - [Creating the credentials file](#creating-the-credentials-file)
  - [Mounting the credentials](#mounting-the-credentials)
  - [Multiple registries](#multiple-registries)
  - [Non-root users](#non-root-users)
- [Using Docker Configs Instead of Bind Mounts](#using-docker-configs-instead-of-bind-mounts)
- [Persistent Repo Storage](#persistent-repo-storage)
- [Production Hardening](#production-hardening)
  - [Read-only Docker socket](#read-only-docker-socket)
  - [Restricting the web UI](#restricting-the-web-ui)
  - [Resource limits](#resource-limits)
  - [Logging](#logging)
- [Multi-Cluster Deployment](#multi-cluster-deployment)
- [Complete Production Example](#complete-production-example)
- [Troubleshooting](#troubleshooting)

---

## Basic Deployment

The simplest SwarmCD deployment mounts the Docker socket directly and exposes the web UI on port 8080:

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
    ports:
      - "8080:8080"
```

```bash
docker stack deploy -c docker-compose.yaml swarm-cd
```

This works well for development and small deployments. For production, read on.

---

## Remote Docker Sockets

By default, SwarmCD talks to the Docker daemon through the local Unix socket at `/var/run/docker.sock`. You can point it to a remote Docker daemon instead using the `DOCKER_HOST` environment variable.

### Using DOCKER_HOST

Set `DOCKER_HOST` to any endpoint that the Docker client understands:

```yaml
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    environment:
      DOCKER_HOST: tcp://remote-manager.example.com:2376
    volumes:
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
```

Common values for `DOCKER_HOST`:

| Value | Description |
|---|---|
| `unix:///var/run/docker.sock` | Local Unix socket (default) |
| `tcp://host:2375` | Unencrypted TCP connection |
| `tcp://host:2376` | TLS-encrypted TCP connection (requires certs) |
| `tcp://socket_proxy:2375` | Docker socket proxy (see below) |

> **Security warning:** Exposing the Docker API over unencrypted TCP (`port 2375`) is dangerous. Use TLS (`port 2376`) for remote connections, or use a socket proxy within the same Swarm overlay network.

### Using a Docker Socket Proxy

A **socket proxy** sits between SwarmCD and the Docker socket, filtering API calls so that only the endpoints SwarmCD needs are accessible. This is the recommended approach for production deployments because it limits the blast radius if SwarmCD is compromised.

[tecnativa/docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy) is a popular choice:

```yaml
version: "3.7"

services:
  socket_proxy:
    image: tecnativa/docker-socket-proxy:0.2.0
    deploy:
      placement:
        constraints:
          - node.role == manager
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      # Read-only endpoints SwarmCD needs
      INFO: 1
      SERVICES: 1
      NETWORKS: 1
      SECRETS: 1
      CONFIGS: 1
      TASKS: 1
      NODES: 1
      # Write endpoints — required for docker stack deploy
      POST: 1
    networks:
      - internal

  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    environment:
      DOCKER_HOST: tcp://socket_proxy:2375
    configs:
      - source: stacks
        target: /app/stacks.yaml
        mode: 0400
      - source: repos
        target: /app/repos.yaml
        mode: 0400
    networks:
      - internal

networks:
  internal:
    driver: overlay

configs:
  stacks:
    file: ./stacks.yaml
  repos:
    file: ./repos.yaml
```

With this setup:

- The **socket proxy** runs on a manager node and is the only container with direct Docker socket access.
- **SwarmCD** connects to the proxy over the internal overlay network — it never touches the Docker socket directly.
- The proxy allows only the API endpoints that SwarmCD needs, rejecting everything else.

### Socket proxy permissions

SwarmCD uses `docker stack deploy` under the hood, which requires access to the following Docker API endpoints:

| Permission | Required | Reason |
|---|---|---|
| `INFO` | Yes | Docker system info |
| `SERVICES` | Yes | Create and update services |
| `NETWORKS` | Yes | Create and manage overlay networks |
| `SECRETS` | Yes | Create and manage Swarm secrets |
| `CONFIGS` | Yes | Create and manage Swarm configs |
| `TASKS` | Yes | Inspect task status |
| `NODES` | Yes | Node information for placement constraints |
| `POST` | Yes | Write operations (deploy, update, remove) |

> **Note:** The exact permissions depend on your stack definitions. If your stacks don't use Swarm secrets or configs, you can disable those endpoints. However, it's generally safest to enable all of the above.

---

## Private Container Registries

If your stacks pull images from private registries, SwarmCD needs credentials to authenticate. SwarmCD passes `--with-registry-auth` to `docker stack deploy`, which forwards registry credentials from the deploying node to worker nodes.

### Creating the credentials file

First, encode your credentials as base64 (use `printf` to avoid trailing newlines):

```bash
printf 'username:password' | base64
```

Then create a Docker config JSON file:

```json
{
    "auths": {
        "registry.example.com": {
            "auth": "dXNlcm5hbWU6cGFzc3dvcmQ="
        }
    }
}
```

Replace `registry.example.com` with your registry hostname and the `auth` value with the base64-encoded output.

> **For Docker Hub:** The registry hostname is `https://index.docker.io/v1/`.

### Mounting the credentials

Mount the config file as a Docker secret at `/root/.docker/config.json`:

```yaml
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
    secrets:
      - source: docker-config
        target: /root/.docker/config.json

secrets:
  docker-config:
    file: ./docker-config.json
```

### Multiple registries

You can authenticate to multiple registries by adding entries to the `auths` object:

```json
{
    "auths": {
        "registry.example.com": {
            "auth": "dXNlcm5hbWU6cGFzc3dvcmQ="
        },
        "ghcr.io": {
            "auth": "Z2l0aHViOnRva2Vu"
        },
        "https://index.docker.io/v1/": {
            "auth": "ZG9ja2VyaHViOnRva2Vu"
        }
    }
}
```

### Non-root users

The published SwarmCD image runs as `root`. If you build a custom image that runs as a different user, adjust the mount path accordingly:

```yaml
secrets:
  - source: docker-config
    target: /home/appuser/.docker/config.json
```

The path must be `~/.docker/config.json` for whichever user the process runs as.

---

## Using Docker Configs Instead of Bind Mounts

For Swarm-native deployments, you can use Docker configs instead of bind-mounting files from the host. This avoids the requirement that the config files exist on the specific node where SwarmCD runs.

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
      DOCKER_HOST: tcp://socket_proxy:2375
    configs:
      - source: repos-config
        target: /app/repos.yaml
        mode: 0400
      - source: stacks-config
        target: /app/stacks.yaml
        mode: 0400
      - source: global-config
        target: /app/config.yaml
        mode: 0400

configs:
  repos-config:
    file: ./repos.yaml
  stacks-config:
    file: ./stacks.yaml
  global-config:
    file: ./config.yaml
```

**Trade-offs:**

| Approach | Pros | Cons |
|---|---|---|
| **Bind mounts** | Easy to edit in place; changes take effect on container restart | Files must exist on the node where the task runs |
| **Docker configs** | Distributed by Swarm; node-independent | Must redeploy the stack (or create a new config version) to change values |

> **Tip:** Use Docker configs for production and bind mounts for development.

---

## Persistent Repo Storage

By default, SwarmCD clones repositories into the `repos/` directory inside the container. This directory is ephemeral — if the container restarts, all repos are re-cloned from scratch. For large repositories, this can be slow.

You can mount a Docker volume to persist cloned repos across restarts:

```yaml
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
      - repo-data:/app/repos

volumes:
  repo-data:
```

If a repo directory already exists when SwarmCD starts, it opens the existing clone instead of cloning again. This makes restarts significantly faster for large repositories.

> **Note:** If you change a repo's URL in `repos.yaml`, you'll need to delete the corresponding subdirectory in the volume (or delete the entire volume) so SwarmCD can clone from the new URL.

---

## Production Hardening

### Read-only Docker socket

Always mount the Docker socket as read-only (`:ro`) when SwarmCD accesses it directly. SwarmCD communicates with Docker through the client library, which uses the socket in read-write mode at the protocol level — the `:ro` mount flag prevents the container from deleting or replacing the socket file itself, which is a small but worthwhile security measure.

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

For stronger isolation, use a [socket proxy](#using-a-docker-socket-proxy).

### Restricting the web UI

The SwarmCD web UI and API listen on the address configured in `config.yaml` (default: `0.0.0.0:8080`). In production, you may want to:

**Option 1: Don't expose the port publicly**

Remove the `ports:` section entirely. The UI will only be accessible from within the Swarm overlay network.

**Option 2: Bind to localhost**

If you only need access from the manager node:

```yaml
# config.yaml
address: 127.0.0.1:8080
```

**Option 3: Put it behind a reverse proxy**

Use Traefik, Nginx, or Caddy as a reverse proxy with authentication in front of SwarmCD. Don't expose port 8080 directly.

### Resource limits

Set resource limits to prevent SwarmCD from consuming excessive resources:

```yaml
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.50"
        reservations:
          memory: 128M
          cpus: "0.25"
```

Adjust based on the number and size of repositories you manage.

### Logging

Configure log verbosity with the `LOG_LEVEL` environment variable:

| Level | What's logged |
|---|---|
| `debug` | Everything — Git operations, file reads, template rendering, deploy commands |
| `info` | Sync cycle start/end, stack updates, errors (default) |
| `warn` | Warnings and errors only |
| `error` | Errors only |

```yaml
environment:
  LOG_LEVEL: info
```

Use `debug` for initial setup and troubleshooting. Use `info` or `warn` for production to keep log volume manageable.

You can also configure Docker's logging driver for the SwarmCD service:

```yaml
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
```

---

## Multi-Cluster Deployment

You can run a single SwarmCD instance that manages stacks across multiple Docker Swarm clusters by deploying multiple SwarmCD services, each pointing to a different `DOCKER_HOST`:

```yaml
version: "3.7"

services:
  swarm-cd-cluster-a:
    image: ghcr.io/m-adawi/swarm-cd:latest
    environment:
      DOCKER_HOST: tcp://cluster-a-manager:2376
    configs:
      - source: repos
        target: /app/repos.yaml
        mode: 0400
      - source: stacks-cluster-a
        target: /app/stacks.yaml
        mode: 0400

  swarm-cd-cluster-b:
    image: ghcr.io/m-adawi/swarm-cd:latest
    environment:
      DOCKER_HOST: tcp://cluster-b-manager:2376
    configs:
      - source: repos
        target: /app/repos.yaml
        mode: 0400
      - source: stacks-cluster-b
        target: /app/stacks.yaml
        mode: 0400

configs:
  repos:
    file: ./repos.yaml
  stacks-cluster-a:
    file: ./stacks-cluster-a.yaml
  stacks-cluster-b:
    file: ./stacks-cluster-b.yaml
```

Each SwarmCD instance has its own `stacks.yaml` defining which stacks to deploy to that cluster, while sharing the same `repos.yaml`.

---

## Complete Production Example

This example combines a socket proxy, private registry auth, SOPS decryption with age, Docker configs, persistent repo storage, and resource limits:

```yaml
version: "3.7"

services:
  socket_proxy:
    image: tecnativa/docker-socket-proxy:0.2.0
    deploy:
      placement:
        constraints:
          - node.role == manager
      resources:
        limits:
          memory: 64M
          cpus: "0.10"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      INFO: 1
      SERVICES: 1
      NETWORKS: 1
      SECRETS: 1
      CONFIGS: 1
      TASKS: 1
      NODES: 1
      POST: 1
    networks:
      - internal
    logging:
      driver: json-file
      options:
        max-size: "5m"
        max-file: "2"

  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.50"
        reservations:
          memory: 128M
          cpus: "0.25"
    environment:
      DOCKER_HOST: tcp://socket_proxy:2375
      SOPS_AGE_KEY_FILE: /secrets/age.key
      LOG_LEVEL: info
    configs:
      - source: repos-config
        target: /app/repos.yaml
        mode: 0400
      - source: stacks-config
        target: /app/stacks.yaml
        mode: 0400
      - source: global-config
        target: /app/config.yaml
        mode: 0400
    secrets:
      - source: age-key
        target: /secrets/age.key
      - source: docker-registry-config
        target: /root/.docker/config.json
    volumes:
      - repo-data:/app/repos
    networks:
      - internal
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

networks:
  internal:
    driver: overlay

volumes:
  repo-data:

configs:
  repos-config:
    file: ./repos.yaml
  stacks-config:
    file: ./stacks.yaml
  global-config:
    file: ./config.yaml

secrets:
  age-key:
    file: ./age.key
  docker-registry-config:
    file: ./docker-config.json
```

**config.yaml:**

```yaml
update_interval: 60
auto_rotate: true
sops_secrets_discovery: true
address: 0.0.0.0:8080
```

---

## Troubleshooting

### "could not create a docker cli object" / "could not initialize docker cli object"

SwarmCD cannot connect to the Docker daemon. Possible causes:

- **Missing Docker socket mount** — ensure `/var/run/docker.sock` is mounted or `DOCKER_HOST` is set.
- **Socket permissions** — the SwarmCD container must have permission to read the socket. The published image runs as root, so this is usually not an issue unless you're using a custom image.
- **Socket proxy not reachable** — verify the proxy service is running and on the same overlay network as SwarmCD.

### "could not deploy stack ..."

`docker stack deploy` failed. Common causes:

- **Image pull failure** — the image doesn't exist, or the registry requires authentication. See [Private Container Registries](#private-container-registries).
- **Insufficient permissions** — if using a socket proxy, ensure all required endpoints are enabled. See [Socket proxy permissions](#socket-proxy-permissions).
- **Invalid Compose file** — the rendered Compose file has syntax errors. Set `LOG_LEVEL=debug` and check the logs for template rendering issues.

### SwarmCD is slow to start after a restart

If you have many or large repositories, initial cloning can take time. Use [persistent repo storage](#persistent-repo-storage) to avoid re-cloning on every restart.

### Stacks are not updating after a Git push

- Check that the `update_interval` hasn't been set too high. The default is 120 seconds.
- Verify that SwarmCD can reach the Git repository (network access, DNS resolution).
- For private repos, ensure the credentials are still valid (tokens may have expiration dates).
- Check the logs for pull errors: `docker service logs swarm-cd_swarm-cd --follow`

### Web UI is not accessible

- Verify the port is published: `docker service inspect swarm-cd_swarm-cd --format '{{json .Endpoint.Ports}}'`
- Check that `address` in `config.yaml` is set to `0.0.0.0:8080` (not `127.0.0.1:8080`, which only allows localhost access).
- If using a reverse proxy, ensure it can reach SwarmCD on the overlay network.