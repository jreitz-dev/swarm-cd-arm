# Getting Started

This guide walks you through deploying your first stack with SwarmCD. By the end you'll have SwarmCD running on a Docker Swarm cluster, automatically syncing a stack from a Git repository.

## Prerequisites

- A running [Docker Swarm](https://docs.docker.com/engine/swarm/swarm-tutorial/create-swarm/) cluster (even a single-node swarm works)
- Access to a **manager node** where you can run `docker stack deploy`
- A Git repository containing at least one Docker Compose file

> **Tip:** If you don't have your own repo yet, you can use the [swarm-cd-example](https://github.com/m-adawi/swarm-cd-example) repository to follow along.

---

## Step 1: Create the Configuration Files

SwarmCD needs two files to know what to deploy: `repos.yaml` (where your code lives) and `stacks.yaml` (what to deploy from it).

Create a working directory on your Swarm manager node:

```bash
mkdir swarm-cd && cd swarm-cd
```

### repos.yaml

This file tells SwarmCD which Git repositories to watch. Each entry has a unique name and a clone URL.

```yaml
# repos.yaml
swarm-cd-example:
  url: "https://github.com/m-adawi/swarm-cd-example.git"
```

For private repositories, see [Configuration Reference — repos.yaml](configuration.md#reposyaml).

### stacks.yaml

This file declares the stacks you want SwarmCD to deploy. Each stack references a repo defined in `repos.yaml`, a branch, and a path to a Compose file inside that repo.

```yaml
# stacks.yaml
nginx:
  repo: swarm-cd-example
  branch: main
  compose_file: nginx/compose.yaml
```

The key (`nginx`) becomes the Docker Swarm stack name — it's what you'll see in `docker stack ls`.

---

## Step 2: Create the SwarmCD Compose File

Create a `docker-compose.yaml` that deploys SwarmCD itself as a Swarm service:

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

Key points:

| Setting | Why |
|---|---|
| `node.role == manager` | SwarmCD must run on a manager node to execute `docker stack deploy` |
| Docker socket mount | SwarmCD needs access to the Docker API to manage stacks |
| Config file mounts | Mounted read-only (`:ro`) so SwarmCD can read your repo and stack definitions |
| Port 8080 | Exposes the web UI and REST API |

---

## Step 3: Deploy SwarmCD

Run the following command on your Swarm manager node:

```bash
docker stack deploy -c docker-compose.yaml swarm-cd
```

Verify it's running:

```bash
docker service ls
```

You should see a `swarm-cd_swarm-cd` service with 1/1 replicas.

---

## Step 4: Watch It Work

SwarmCD will immediately:

1. **Clone** the `swarm-cd-example` repository
2. **Check out** the `main` branch
3. **Deploy** the `nginx` stack using the Compose file at `nginx/compose.yaml`

After a few seconds, check that the `nginx` stack was deployed:

```bash
docker stack ls
```

You should see both `swarm-cd` and `nginx` in the list.

### View the Dashboard

Open your browser and navigate to:

```
http://<your-manager-ip>:8080
```

The SwarmCD UI shows each managed stack with its:

- **Name** — the stack name from `stacks.yaml`
- **Revision** — the short Git commit hash currently deployed
- **Repo URL** — link to the source repository
- **Error** — any errors from the last sync cycle (empty when healthy)

---

## Step 5: Make a Change

SwarmCD polls the repository on a configurable interval (default: **120 seconds**). To see the sync in action:

1. Push a change to the `compose.yaml` in your repo (for example, change the nginx image tag)
2. Wait for the next sync cycle
3. Refresh the SwarmCD dashboard — the **Revision** will update to the new commit hash

You can lower the polling interval by adding a `config.yaml` file:

```yaml
# config.yaml
update_interval: 30  # poll every 30 seconds
```

Then mount it into the container:

```yaml
volumes:
  - ./config.yaml:/app/config.yaml:ro
```

Redeploy SwarmCD for the change to take effect:

```bash
docker stack deploy -c docker-compose.yaml swarm-cd
```

---

## Step 6: Add More Stacks

To deploy additional stacks, simply add entries to `stacks.yaml`:

```yaml
# stacks.yaml
nginx:
  repo: swarm-cd-example
  branch: main
  compose_file: nginx/compose.yaml

nginx-ssl:
  repo: swarm-cd-example
  branch: main
  compose_file: nginx-ssl/compose.yaml
  sops_files:
    - nginx-ssl/secrets/www.example.com.crt
    - nginx-ssl/secrets/www.example.com.key
```

Then redeploy SwarmCD:

```bash
docker stack deploy -c docker-compose.yaml swarm-cd
```

SwarmCD will pick up the new stack definitions and deploy them on the next cycle.

---

## Viewing Logs

To troubleshoot or monitor SwarmCD, check the service logs:

```bash
docker service logs swarm-cd_swarm-cd --follow
```

You can increase log verbosity by setting the `LOG_LEVEL` environment variable:

```yaml
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    environment:
      LOG_LEVEL: debug
    # ... rest of config
```

Available log levels: `debug`, `info` (default), `warn`, `error`.

---

## File Layout Summary

After completing this guide, your working directory looks like this:

```
swarm-cd/
├── docker-compose.yaml   # SwarmCD's own Compose file
├── repos.yaml            # Git repository definitions
├── stacks.yaml           # Stack definitions
└── config.yaml           # (optional) Global settings
```

---

## What's Next?

Now that you have SwarmCD running, explore these topics:

- **[Configuration Reference](configuration.md)** — learn every available option for all three config files
- **[Secrets Management](secrets.md)** — encrypt secrets with SOPS and have SwarmCD decrypt them automatically
- **[Templating & Env Files](templating.md)** — parameterize Compose files with Go templates or `.env` variable substitution
- **[Deployment Patterns](deployment.md)** — remote Docker sockets, socket proxies, and private registry authentication
- **[Architecture](architecture.md)** — understand how SwarmCD works under the hood