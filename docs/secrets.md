# Secrets Management

SwarmCD integrates with [SOPS](https://github.com/getsops/sops) to let you store encrypted secrets directly in your Git repositories. Before every deploy, SwarmCD decrypts the files you specify (or auto-discovers them), so your Compose stacks always have access to the plaintext values at deploy time — without ever committing plaintext to Git.

---

## Table of Contents

- [How It Works](#how-it-works)
- [Supported Encryption Backends](#supported-encryption-backends)
- [Setting Up with age](#setting-up-with-age)
  - [1. Generate an age key pair](#1-generate-an-age-key-pair)
  - [2. Encrypt your secret files](#2-encrypt-your-secret-files)
  - [3. Commit the encrypted files](#3-commit-the-encrypted-files)
  - [4. Configure SwarmCD](#4-configure-swarmcd)
- [Setting Up with GPG](#setting-up-with-gpg)
  - [Using a GPG key file](#using-a-gpg-key-file)
  - [Using a GPG key from an environment variable](#using-a-gpg-key-from-an-environment-variable)
- [Other Backends (AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault)](#other-backends-aws-kms-gcp-kms-azure-key-vault-hashicorp-vault)
- [Specifying Secret Files Explicitly](#specifying-secret-files-explicitly)
- [Automatic Secret Discovery](#automatic-secret-discovery)
  - [Stack-level discovery](#stack-level-discovery)
  - [Global discovery](#global-discovery)
  - [How discovery works](#how-discovery-works)
  - [Precedence rules](#precedence-rules)
- [Supported File Formats](#supported-file-formats)
- [Complete Example: age + Automatic Discovery](#complete-example-age--automatic-discovery)
- [Complete Example: GPG + Explicit File List](#complete-example-gpg--explicit-file-list)
- [Troubleshooting](#troubleshooting)

---

## How It Works

On every sync cycle, for each stack, SwarmCD performs the following steps in order:

1. **Pulls** the latest changes from the stack's Git repository.
2. **Reads** the Compose file.
3. **Substitutes** environment variables and renders Go templates (if configured).
4. **Decrypts** SOPS-encrypted files — either from an explicit list (`sops_files`) or via automatic discovery (`sops_secrets_discovery`).
5. **Rotates** config and secret names by appending a content hash (if `auto_rotate` is enabled).
6. **Deploys** the stack with `docker stack deploy`.

Decryption happens **in place** on the cloned working copy. Because SwarmCD pulls fresh changes on every cycle, the encrypted originals are always restored from Git before decryption runs again.

---

## Supported Encryption Backends

SwarmCD uses the SOPS library directly, so it supports every backend that SOPS supports:

| Backend | Environment Variable(s) | Notes |
|---|---|---|
| [age](https://github.com/FiloSottile/age) | `SOPS_AGE_KEY_FILE` | Recommended for simplicity |
| GPG | `SOPS_GPG_PRIVATE_KEY_FILE` or `SOPS_GPG_PRIVATE_KEY` | Key is auto-imported at container startup |
| AWS KMS | `AWS_*` credentials | See [SOPS docs](https://github.com/getsops/sops#encrypting-using-aws-kms) |
| GCP KMS | `GOOGLE_APPLICATION_CREDENTIALS` | See [SOPS docs](https://github.com/getsops/sops#encrypting-using-gcp-kms) |
| Azure Key Vault | `AZURE_*` credentials | See [SOPS docs](https://github.com/getsops/sops#encrypting-using-azure-key-vault) |
| HashiCorp Vault | `VAULT_ADDR`, `VAULT_TOKEN` | See [SOPS docs](https://github.com/getsops/sops#encrypting-using-hashicorp-vault) |

You can mix backends in a single `.sops.yaml` creation rule — SOPS handles that natively.

---

## Setting Up with age

[age](https://github.com/FiloSottile/age) is the simplest backend to set up and is the recommended choice for most deployments.

### 1. Generate an age key pair

On your local machine (not the server):

```bash
age-keygen -o age.key
```

This creates a file containing both the private key (the `AGE-SECRET-KEY-...` line) and the public key (in a comment). Note the public key — you'll need it for encryption.

### 2. Encrypt your secret files

Use `sops` with the age public key to encrypt files:

```bash
# Encrypt a YAML file
sops --encrypt --age age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --input-type yaml --output-type yaml \
  secrets/db-password.yaml > secrets/db-password.enc.yaml

# Encrypt a binary file (e.g. a TLS certificate)
sops --encrypt --age age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --input-type binary --output-type binary \
  secrets/tls.crt > secrets/tls.crt.enc
```

Or create a `.sops.yaml` at the repo root so you don't have to specify the key every time:

```yaml
# .sops.yaml (in your stack repo)
creation_rules:
  - age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Then simply:

```bash
sops --encrypt --in-place secrets/db-password.yaml
sops --encrypt --in-place secrets/tls.crt
```

### 3. Commit the encrypted files

```bash
git add secrets/
git commit -m "Add encrypted secrets"
git push
```

### 4. Configure SwarmCD

Mount the age private key as a Docker secret and tell SwarmCD where to find it:

```yaml
# docker-compose.yaml (SwarmCD deployment)
version: "3.7"
services:
  swarm-cd:
    image: ghcr.io/m-adawi/swarm-cd:latest
    deploy:
      placement:
        constraints:
          - node.role == manager
    environment:
      SOPS_AGE_KEY_FILE: /secrets/age.key
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
    secrets:
      - source: age-key
        target: /secrets/age.key

secrets:
  age-key:
    file: ./age.key
```

Then reference the encrypted files in your stack definition:

```yaml
# stacks.yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  sops_files:
    - secrets/db-password.yaml
    - secrets/tls.crt
```

---

## Setting Up with GPG

SwarmCD's entrypoint script automatically imports GPG keys into the container's keyring at startup. There are two ways to provide the key.

### Using a GPG key file

Export your GPG private key to a file:

```bash
gpg --export-secret-keys --armor your-key-id > private.gpg
```

Mount it as a Docker secret and set `SOPS_GPG_PRIVATE_KEY_FILE`:

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
      SOPS_GPG_PRIVATE_KEY_FILE: /secrets/private.gpg
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
    secrets:
      - source: gpg-key
        target: /secrets/private.gpg

secrets:
  gpg-key:
    file: ./private.gpg
```

At startup, the entrypoint script runs `gpg --import` on the file. You'll see a log message confirming the import.

### Using a GPG key from an environment variable

If you prefer not to use a file, you can pass the GPG key directly in the `SOPS_GPG_PRIVATE_KEY` environment variable:

```yaml
environment:
  SOPS_GPG_PRIVATE_KEY: |
    -----BEGIN PGP PRIVATE KEY BLOCK-----
    ... (your key here) ...
    -----END PGP PRIVATE KEY BLOCK-----
```

> **Warning:** Embedding the key directly in a Compose file is less secure. Use Docker secrets or an external secrets manager when possible.

The entrypoint script checks for `SOPS_GPG_PRIVATE_KEY_FILE` first. If it's not set, it falls back to `SOPS_GPG_PRIVATE_KEY`. If neither is set, GPG import is skipped (which is fine if you're using age or another backend).

---

## Other Backends (AWS KMS, GCP KMS, Azure Key Vault, HashiCorp Vault)

SOPS supports several cloud KMS backends. Since SwarmCD uses the SOPS library directly, all of these work — you just need to provide the appropriate credentials as environment variables on the SwarmCD container.

For example, to use **AWS KMS**:

```yaml
environment:
  AWS_ACCESS_KEY_ID: AKIA...
  AWS_SECRET_ACCESS_KEY: ...
  AWS_DEFAULT_REGION: us-east-1
```

For **HashiCorp Vault**:

```yaml
environment:
  VAULT_ADDR: https://vault.example.com
  VAULT_TOKEN: s.xxxxxxxxxxxxx
```

Refer to the [SOPS documentation](https://github.com/getsops/sops#usage) for the full list of environment variables each backend requires.

---

## Specifying Secret Files Explicitly

The `sops_files` field in `stacks.yaml` accepts a list of file paths relative to the repository root. SwarmCD decrypts each file in place before deploying the stack.

```yaml
# stacks.yaml
nginx-ssl:
  repo: my-repo
  branch: main
  compose_file: nginx-ssl/compose.yaml
  sops_files:
    - nginx-ssl/secrets/tls.crt
    - nginx-ssl/secrets/tls.key
    - nginx-ssl/secrets/dhparam.pem
```

This approach gives you full control over exactly which files are decrypted. Use it when:

- Only some files in the repo are encrypted
- You want to be explicit about what's being decrypted
- The encrypted files are not referenced in the Compose file's `secrets:` section (e.g. they're used as config files or mounted as bind mounts)

---

## Automatic Secret Discovery

Instead of listing every file manually, you can let SwarmCD figure out which files to decrypt by inspecting the Compose file's `secrets:` section.

### Stack-level discovery

Enable it on individual stacks:

```yaml
# stacks.yaml
my-app:
  repo: my-repo
  branch: main
  compose_file: app/compose.yaml
  sops_secrets_discovery: true
```

When this is set to `true`, the `sops_files` field for this stack is **ignored**.

### Global discovery

Enable it for all stacks at once in `config.yaml`:

```yaml
# config.yaml
sops_secrets_discovery: true
```

When global discovery is enabled, **all** per-stack `sops_secrets_discovery` and `sops_files` settings are ignored.

### How discovery works

SwarmCD parses the Compose file and looks at the top-level `secrets:` section. For each secret:

1. If the secret has `external: true`, it is **skipped** (nothing to decrypt).
2. Otherwise, SwarmCD reads the `file:` field and resolves the path relative to the Compose file's directory.
3. The resolved file is added to the decryption list.

**Example Compose file:**

```yaml
# app/compose.yaml
version: "3.7"
services:
  web:
    image: myapp:latest
    secrets:
      - db_password
      - api_key

secrets:
  db_password:
    file: ./secrets/db-password.txt      # ← will be decrypted
  api_key:
    file: ./secrets/api-key.txt          # ← will be decrypted
  existing_secret:
    external: true                       # ← skipped
```

With discovery enabled, SwarmCD will automatically decrypt `app/secrets/db-password.txt` and `app/secrets/api-key.txt` before deploying.

### Precedence rules

| Global `sops_secrets_discovery` | Stack `sops_secrets_discovery` | Stack `sops_files` | What happens |
|---|---|---|---|
| `true` | *(ignored)* | *(ignored)* | Auto-discover for **all** stacks |
| `false` | `true` | *(ignored)* | Auto-discover for **this stack** only |
| `false` | `false` | `[file1, file2]` | Decrypt **only** the listed files |
| `false` | `false` | `[]` or not set | **No** SOPS decryption |

---

## Supported File Formats

SOPS handles different file formats differently. SwarmCD determines the format from the file extension:

| Extension | SOPS Format | Notes |
|---|---|---|
| `.yaml`, `.yml` | `yaml` | Structured encryption — only values are encrypted, keys remain readable |
| `.json` | `json` | Structured encryption — only values are encrypted |
| `.ini` | `ini` | Structured encryption |
| `.env` | `dotenv` | Structured encryption — values are encrypted, keys remain readable |
| *(anything else)* | `binary` | The entire file is encrypted as a single blob |

For TLS certificates, private keys, and other non-structured files, SOPS uses **binary** mode. The entire file contents are encrypted and stored as a base64 blob.

---

## Complete Example: age + Automatic Discovery

This example shows a full production setup with age encryption and automatic secret discovery.

**Repository structure:**

```
my-stacks/
├── .sops.yaml
└── webapp/
    ├── compose.yaml
    └── secrets/
        ├── db-password.txt    (encrypted with sops)
        └── api-key.txt        (encrypted with sops)
```

**.sops.yaml:**

```yaml
creation_rules:
  - age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**webapp/compose.yaml:**

```yaml
version: "3.7"
services:
  app:
    image: myapp:latest
    secrets:
      - db_password
      - api_key

secrets:
  db_password:
    file: ./secrets/db-password.txt
  api_key:
    file: ./secrets/api-key.txt
```

**repos.yaml:**

```yaml
my-stacks:
  url: "https://github.com/you/my-stacks.git"
```

**stacks.yaml:**

```yaml
webapp:
  repo: my-stacks
  branch: main
  compose_file: webapp/compose.yaml
  sops_secrets_discovery: true
```

**docker-compose.yaml (SwarmCD deployment):**

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
      SOPS_AGE_KEY_FILE: /secrets/age.key
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
    secrets:
      - source: age-key
        target: /secrets/age.key

secrets:
  age-key:
    file: ./age.key
```

---

## Complete Example: GPG + Explicit File List

This example shows GPG encryption with an explicit list of files to decrypt.

**stacks.yaml:**

```yaml
nginx-ssl:
  repo: my-stacks
  branch: main
  compose_file: nginx-ssl/compose.yaml
  sops_files:
    - nginx-ssl/secrets/www.example.com.crt
    - nginx-ssl/secrets/www.example.com.key
```

**docker-compose.yaml (SwarmCD deployment):**

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
      SOPS_GPG_PRIVATE_KEY_FILE: /secrets/private.gpg
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./repos.yaml:/app/repos.yaml:ro
      - ./stacks.yaml:/app/stacks.yaml:ro
    secrets:
      - source: gpg-key
        target: /secrets/private.gpg

secrets:
  gpg-key:
    file: ./private.gpg
```

---

## Troubleshooting

### "could not decrypt the file ..."

- **Wrong key:** Verify that the key file mounted into the container matches the key used for encryption. For age, the public key in your `.sops.yaml` must correspond to the private key in your `age.key` file.
- **Missing environment variable:** Ensure `SOPS_AGE_KEY_FILE` or `SOPS_GPG_PRIVATE_KEY_FILE` is set and points to the correct path inside the container.
- **File not found:** Check that the path in `sops_files` or the `file:` field in your Compose `secrets:` section is correct and relative to the repo root (for `sops_files`) or the Compose file directory (for auto-discovery).

### "entrypoint.sh: error: could not import GPG private key from file !"

The GPG key file is either malformed, empty, or not accessible. Verify:

- The file exists at the path specified by `SOPS_GPG_PRIVATE_KEY_FILE`
- The file contains a valid ASCII-armored GPG private key
- The Docker secret is correctly mounted

### Secrets are not being decrypted

- If you're using automatic discovery, ensure `sops_secrets_discovery` is set to `true` either globally in `config.yaml` or on the specific stack in `stacks.yaml`.
- If you're using explicit file lists, check that the paths in `sops_files` are correct and relative to the repository root.
- Set `LOG_LEVEL=debug` to see detailed logs about which files SwarmCD attempts to decrypt.

### Secrets appear as encrypted text in deployed services

This usually means decryption is silently failing or not running at all. Check the SwarmCD logs:

```bash
docker service logs swarm-cd_swarm-cd --follow
```

Look for error messages mentioning "decrypt" or "sops". Enable debug logging for more detail:

```yaml
environment:
  LOG_LEVEL: debug
```
