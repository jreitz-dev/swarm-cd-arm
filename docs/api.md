# REST API Reference

SwarmCD exposes a lightweight REST API for querying the status of managed stacks. The API is served by the same HTTP server as the web UI, on the address configured via `address` in `config.yaml` (default: `0.0.0.0:8080`).

---

## Table of Contents

- [Overview](#overview)
- [Base URL](#base-url)
- [Endpoints](#endpoints)
  - [GET /stacks](#get-stacks)
  - [GET /](#get-)
  - [GET /ui](#get-ui)
  - [GET /assets/\*](#get-assets)
- [Response Format](#response-format)
- [Error Handling](#error-handling)
- [Usage Examples](#usage-examples)
  - [curl](#curl)
  - [Python](#python)
  - [Bash monitoring script](#bash-monitoring-script)
- [Integration Ideas](#integration-ideas)

---

## Overview

The API is **read-only** — there are no endpoints to trigger deployments, modify configuration, or manage stacks. SwarmCD's deployment behavior is driven entirely by Git repository changes and configuration files.

All API responses use `application/json` content type. There is **no authentication** on the API by default — see [Deployment Patterns](deployment.md#restricting-the-web-ui) for options to restrict access.

---

## Base URL

```
http://<swarmcd-host>:<port>
```

The default port is `8080`. You can change it via the `address` field in `config.yaml`:

```yaml
# config.yaml
address: 0.0.0.0:9090
```

---

## Endpoints

### GET /stacks

Returns the current status of all managed stacks.

**Request:**

```
GET /stacks HTTP/1.1
Host: <swarmcd-host>:8080
```

No query parameters, request body, or authentication headers are required.

**Response:**

- **Status:** `200 OK`
- **Content-Type:** `application/json`

**Response body:**

A JSON array of stack status objects, sorted alphabetically by `Name`.

```json
[
  {
    "Name": "my-app",
    "Revision": "a1b2c3d4",
    "RepoURL": "https://github.com/you/my-stacks.git",
    "Error": ""
  },
  {
    "Name": "monitoring",
    "Revision": "e5f6a7b8",
    "RepoURL": "https://github.com/you/infra.git",
    "Error": ""
  },
  {
    "Name": "nginx",
    "Revision": "",
    "RepoURL": "https://github.com/you/my-stacks.git",
    "Error": "could not pull main branch in my-stacks repo: authentication failed"
  }
]
```

**Fields:**

| Field | Type | Description |
|---|---|---|
| `Name` | string | The stack name as defined in `stacks.yaml`. This is also the Docker Swarm stack name used in `docker stack deploy`. |
| `Revision` | string | The short (8-character) Git commit hash of the last successfully deployed revision. Empty string if the stack has never been successfully deployed. |
| `RepoURL` | string | The Git clone URL of the repository that contains this stack's Compose file. |
| `Error` | string | The error message from the most recent sync cycle. Empty string if the last sync was successful. Only the most recent error is stored — previous errors are overwritten. |

**Notes:**

- The response array is **always** returned, even if it's empty (`[]`).
- Stacks are sorted **alphabetically** by the `Name` field.
- The `Revision` field is updated only on successful deploys. If a stack fails to deploy, the revision retains the value from the last successful deploy.
- The `Error` field is cleared on the next successful sync. It only reflects the **most recent** cycle's result.

---

### GET /

Redirects to the web UI.

**Request:**

```
GET / HTTP/1.1
```

**Response:**

- **Status:** `302 Found`
- **Location:** `/ui`

---

### GET /ui

Serves the SwarmCD web UI (the React single-page application).

**Request:**

```
GET /ui HTTP/1.1
```

**Response:**

- **Status:** `200 OK`
- **Content-Type:** `text/html`
- **Body:** The `index.html` file of the built React application.

---

### GET /assets/*

Serves static assets (JavaScript, CSS, images) for the web UI.

**Request:**

```
GET /assets/index-abc123.js HTTP/1.1
```

**Response:**

- **Status:** `200 OK`
- **Content-Type:** Varies by file type
- **Body:** The requested static file.

---

## Response Format

### Successful stack status

A stack that is syncing correctly will have an empty `Error` field and a populated `Revision`:

```json
{
  "Name": "webapp",
  "Revision": "c3d4e5f6",
  "RepoURL": "https://github.com/you/stacks.git",
  "Error": ""
}
```

### Stack with an error

A stack that encountered an error during the last sync cycle will have a non-empty `Error` field. The `Revision` may still contain the last successfully deployed revision:

```json
{
  "Name": "broken-stack",
  "Revision": "a1b2c3d4",
  "RepoURL": "https://github.com/you/stacks.git",
  "Error": "could not decrypt the file secrets/api-key.txt: cipher: message authentication failed"
}
```

### Stack that has never been deployed

A stack that has never had a successful deploy will have an empty `Revision`:

```json
{
  "Name": "new-stack",
  "Revision": "",
  "RepoURL": "https://github.com/you/stacks.git",
  "Error": "could not read compose file app/compose.yaml: open repos/my-repo/app/compose.yaml: no such file or directory"
}
```

---

## Error Handling

The `/stacks` endpoint itself always returns `200 OK` as long as SwarmCD is running. Per-stack errors are reported **inside** the response body via the `Error` field — they are not reflected in the HTTP status code.

If SwarmCD is unreachable (e.g., the service is down or the port is not published), you'll receive a connection error from your HTTP client, not an API error response.

---

## Usage Examples

### curl

Fetch all stack statuses:

```bash
curl -s http://localhost:8080/stacks | jq .
```

Check if any stacks have errors:

```bash
curl -s http://localhost:8080/stacks | jq '[.[] | select(.Error != "")]'
```

Get the revision of a specific stack:

```bash
curl -s http://localhost:8080/stacks | jq -r '.[] | select(.Name == "my-app") | .Revision'
```

### Python

```python
import requests

response = requests.get("http://localhost:8080/stacks")
stacks = response.json()

for stack in stacks:
    status = "ERROR" if stack["Error"] else "OK"
    print(f"{stack['Name']}: {status} (rev: {stack['Revision']})")

# Find stacks with errors
errors = [s for s in stacks if s["Error"]]
if errors:
    print(f"\n{len(errors)} stack(s) have errors:")
    for s in errors:
        print(f"  - {s['Name']}: {s['Error']}")
```

### Bash monitoring script

A simple script that checks SwarmCD periodically and sends an alert if any stacks have errors:

```bash
#!/bin/bash
SWARMCD_URL="http://localhost:8080/stacks"

errors=$(curl -sf "$SWARMCD_URL" | jq -r '.[] | select(.Error != "") | "\(.Name): \(.Error)"')

if [ -n "$errors" ]; then
    echo "SwarmCD stack errors detected:"
    echo "$errors"
    # Send alert via your preferred method (email, Slack, etc.)
fi
```

---

## Integration Ideas

The `/stacks` API is intentionally simple, making it easy to integrate with external tools:

| Use Case | Approach |
|---|---|
| **Health check** | Poll `/stacks` and check that no stacks have a non-empty `Error` field. |
| **Monitoring dashboard** | Ingest `/stacks` into Grafana, Datadog, or similar via a custom exporter or script. |
| **Slack/Discord alerts** | Run a cron job or sidecar that polls `/stacks` and posts to a webhook when errors appear. |
| **CI/CD verification** | After pushing to Git, poll `/stacks` until the `Revision` matches the expected commit hash. |
| **Inventory** | Use `/stacks` to build a list of all deployed stacks, their source repos, and current versions. |

> **Note:** The API does not support webhook-style push notifications. You need to poll the endpoint to detect changes. A polling interval of 30–60 seconds is reasonable for most monitoring use cases.