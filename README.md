# ⚓ Vessel

**Lightweight self-hosted app deployment manager for Linux VPS.**

Deploy and manage popular self-hosted applications with minimal DevOps knowledge. Single Go binary, no cloud dependency, no external backend.

---

## Install

```bash
curl -sSL https://raw.githubusercontent.com/Yash121l/Vessel/main/install.sh | sudo bash
```

Then open `http://your-server-ip:4800` in your browser.

On first launch, Vessel asks you to create an owner account. All management API
routes are protected after setup.

---

## What it does

Vessel is an operating layer for self-hosted apps on a VPS. It:

- Bootstraps your server (Docker, Caddy, firewall)
- Deploys apps from curated templates with one click
- Generates Docker Compose files automatically
- Configures Caddy reverse proxy with automatic HTTPS
- Streams live logs from any deployment
- Manages start/stop/restart/update lifecycle
- Imports and monitors existing Docker containers
- Provides a small role-based user system for teams sharing one VPS

---

## Supported Apps

The v0.1 path focuses on a small reliable catalog first. Additional templates can
be added from `/var/lib/vessel/templates`, but new built-ins should be verified on
a real VPS before being treated as stable.

| App | Category | Description |
|-----|----------|-------------|
| **Metabase** | Analytics | Business intelligence & dashboards |
| **n8n** | Automation | Visual workflow automation |
| **Umami** | Analytics | Privacy-friendly web analytics |
| **Plausible** | Analytics | Lightweight Google Analytics alternative |
| **Open WebUI** | AI | Interface for Ollama & OpenAI |
| **Plane** | Productivity | Project management (Jira/Linear alternative) |

---

## Architecture

```
vessel/
├── main.go                     # Entry point
├── internal/
│   ├── cli/                    # Cobra CLI commands
│   │   ├── root.go             # Root command + serve/bootstrap/version
│   │   ├── bootstrap.go        # System bootstrap logic
│   │   └── version.go          # Version constant
│   ├── config/
│   │   └── config.go           # Config loading (file + env)
│   ├── store/
│   │   ├── db.go               # SQLite connection + migrations
│   │   └── deployments.go      # Deployment CRUD + settings
│   ├── registry/
│   │   ├── template.go         # AppTemplate type + Registry
│   │   └── builtins.go         # Built-in app definitions
│   ├── deployment/
│   │   ├── compose.go          # Docker Compose file generation
│   │   └── engine.go           # Deploy/start/stop/update/logs
│   ├── proxy/
│   │   └── manager.go          # Caddy config generation + reload
│   ├── nginx/                  # Advanced/experimental host nginx inspection
│   └── server/
│       ├── server.go           # HTTP server setup + graceful shutdown
│       ├── routes.go           # REST API handlers
│       └── ui.go               # Embedded single-page UI
├── templates/                  # YAML app templates
│   ├── metabase.yaml
│   ├── n8n.yaml
│   ├── umami.yaml
│   ├── plausible.yaml
│   ├── open-webui.yaml
│   └── plane.yaml
├── install.sh                  # One-line installer
├── vessel.service              # systemd unit file
└── Makefile
```

---

## REST API

All endpoints are under `/api/v1`.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/apps` | List available app templates |
| `GET` | `/apps/:id` | Get a specific template |
| `GET` | `/setup` | Check whether first-run setup is complete |
| `POST` | `/setup` | Create the first owner account |
| `POST` | `/login` | Start a user session |
| `POST` | `/logout` | End the current user session |
| `GET` | `/me` | Get the current signed-in user |
| `GET` | `/users` | List users (admin/owner) |
| `POST` | `/users` | Create a user (admin/owner) |
| `PUT` | `/users/:id` | Update a role or password (admin/owner) |
| `DELETE` | `/users/:id` | Delete a user (admin/owner) |
| `GET` | `/deployments` | List all deployments |
| `POST` | `/deployments` | Create a new deployment |
| `GET` | `/deployments/:id` | Get deployment details |
| `DELETE` | `/deployments/:id` | Remove a deployment |
| `POST` | `/deployments/:id/start` | Start a stopped deployment |
| `POST` | `/deployments/:id/stop` | Stop a running deployment |
| `POST` | `/deployments/:id/restart` | Restart a deployment |
| `POST` | `/deployments/:id/update` | Pull latest images and recreate |
| `GET` | `/deployments/:id/logs` | Stream logs (SSE) |
| `GET` | `/health` | Health check |

All endpoints except `/setup`, `/login`, and `/health` require a user session
cookie or a bearer token matching the current session token.

### Roles

Vessel usually runs as root so it can manage Docker, proxy config, and system
services. The UI therefore uses application-level roles as an easy management
rail:

| Role | Access |
|------|--------|
| **viewer** | Read apps, containers, Compose details, and logs |
| **operator** | Viewer access plus deploy, start, stop, restart, update, import |
| **admin** | Operator access plus settings, advanced host proxy tools, and user management for non-owner users |
| **owner** | Full access, including creating or modifying owner users |

These roles do not create Linux users or OS-level isolation. They limit what a
signed-in Vessel user can do through the web UI and API.

### Deploy an app (example)

```bash
curl -X POST http://localhost:4800/api/v1/deployments \
  -H 'Content-Type: application/json' \
  -d '{
    "app_id": "n8n",
    "name": "my-n8n",
    "domain": "n8n.example.com",
    "env": {
      "N8N_BASIC_AUTH_PASSWORD": "supersecret",
      "N8N_ENCRYPTION_KEY": "another-secret-key"
    }
  }'
```

---

## Configuration

`/etc/vessel/config.yaml`:

```yaml
port: 4800
data_dir: /var/lib/vessel
```

Environment variable overrides:
- `VESSEL_CONFIG` — path to config file
- `VESSEL_PORT` — UI port
- `VESSEL_DATA_DIR` — data directory

---

## Development

```bash
# Build
make build

# Run locally (no root needed for dev)
VESSEL_DATA_DIR=./data VESSEL_PORT=4800 ./vessel serve

# Hot reload (requires air: go install github.com/air-verse/air@latest)
make dev

# Cross-compile release binaries
make release

# All make targets
make help
```

> **Note:** The project uses `GOFLAGS="-mod=mod"` due to the CGO dependency (`go-sqlite3`). This is handled automatically by `make`.---

## Data layout

```
/var/lib/vessel/
├── vessel.db              # SQLite metadata
├── deployments/
│   └── my-n8n/
│       ├── docker-compose.yml
│       └── .env
├── templates/             # Custom YAML templates (optional)
└── caddy/
    ├── Caddyfile          # Main Caddy config
    └── sites/
        └── n8n_example_com.caddy
```

---

## Step One Scope

Vessel's first milestone is reliable single-server app deployment:

- Caddy is the primary reverse proxy path.
- First-run owner setup protects host-level controls.
- Role-based users provide clear rails for shared access to a root-powered UI.
- Deployment names, domains, ports, env keys, Docker images, and config filenames
  are validated server-side.
- Secret-looking env values are redacted from API responses.
- Domain deployments bind app ports to `127.0.0.1` so Caddy can reach them
  without publicly exposing those ports.
- The UI starts from apps/deployments, generated template fields, logs, and
  Compose visibility. Advanced nginx management is not part of the primary flow.

---

## Non-goals

Vessel deliberately does not support: Kubernetes, multi-node orchestration, teams,
SaaS features, CI/CD pipelines, GitOps, cloud sync, or enterprise features.

---

## License

MIT
