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

- Bootstraps your server (Docker, nginx, firewall)
- Deploys apps from curated templates with one click
- Pulls a public template catalog so new templates can be used without a binary upgrade
- Generates Docker Compose files automatically
- Configures nginx reverse proxy routes and certificates for domain-based apps
- Streams live logs from any deployment
- Manages start/stop/restart/update lifecycle
- Tracks long-running operations with step-level history
- Creates full backup archives and restores them with the CLI
- Imports and monitors existing Docker containers
- Provides a small role-based user system for teams sharing one VPS

---

## Supported Apps

Vessel ships with an embedded YAML catalog and can also pull the latest public
catalog from GitHub Pages at startup. Additional local templates can be added
from `/var/lib/vessel/templates`; local templates override bundled and remote
templates with the same ID.

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
│   ├── backup/                 # Full backup / restore archive support
│   ├── nginx/                  # Host nginx management and proxy config
│   ├── operations/             # Background operation runner + step logging
│   ├── proxy/                  # Legacy Caddy helper kept for compatibility
│   └── server/
│       ├── server.go           # HTTP server setup + graceful shutdown
│       ├── routes.go           # REST API handlers
│       ├── ui.go               # Embedded UI loader
│       └── ui/                 # Embedded single-page app source
├── internal/registry/templates/ # Single source for app YAML templates
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

The GitHub Pages site lives in `docs/`. The Pages workflow publishes docs plus a
runtime template catalog at `templates/index.json`, generated from
`internal/registry/templates/`.

### Template Contribution Flow

Add or update one YAML file in `internal/registry/templates/`. That single file
is embedded in new binaries and published to GitHub Pages for existing installs.
The remote catalog includes the YAML payload in `index.json`, so Vessel normally
needs one small HTTP request at startup instead of fetching every template file
one by one. Local templates in `/var/lib/vessel/templates` still override bundled
and remote entries for private or experimental deployments.

See `CONTRIBUTING.md` for the full template checklist.

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
| `GET` | `/operations` | List recent tracked operations |
| `GET` | `/operations/:id` | Get one operation with step history |
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
| `GET` | `/system/backups` | List local backup archives |
| `POST` | `/system/backups` | Create a new backup archive |
| `GET` | `/health` | Health check |

All endpoints except `/setup`, `/login`, and `/health` require a user session
cookie or a bearer token matching the current session token.

Deployment and backup creation endpoints now return a tracked operation object.
The UI follows these operations until they succeed or fail and records the
step-level history under `/operations`.

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
- `VESSEL_TEMPLATE_CATALOG_URL` — remote template catalog URL
- `VESSEL_TEMPLATE_CATALOG_DISABLED=1` — skip remote template loading

### Debug Logging

To troubleshoot or run Vessel in debug mode, append the `--debug` flag to any command (e.g. `vessel serve --debug` or `vessel update --debug`).

When run with `--debug`:
- Detailed trace logs (including database transactions, reverse proxy configurations, and raw Docker Compose commands) are printed to the terminal.
- An extensive log file `vessel.log` is generated at your configured `data_dir` (defaulting to `/var/lib/vessel/vessel.log`).
- For normal runs without `--debug`, no log files are created, maintaining a completely clean system.

---

## Development

```bash
# Create a full runtime backup
./vessel backup

# Restore a backup onto a fresh machine
./vessel restore /var/lib/vessel/backups/vessel-backup-YYYYMMDD-HHMMSS.tar.gz

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
├── backups/               # Generated backup archives
├── deployments/
│   └── my-n8n/
│       ├── docker-compose.yml
│       └── .env
├── templates/             # Custom YAML templates (optional)
```

Nginx configuration stays in the host nginx tree, typically under `/etc/nginx`.
Backups include the Vessel data directory, config file, and nginx config tree.

---

## Step One Scope

Vessel's first milestone is reliable single-server app deployment:

- Nginx is the active reverse proxy path.
- First-run owner setup protects host-level controls.
- Role-based users provide clear rails for shared access to a root-powered UI.
- Deployment names, domains, ports, env keys, Docker images, and config filenames
  are validated server-side.
- Secret-looking env values are redacted from API responses.
- Domain deployments bind app ports to `127.0.0.1` so nginx can reach them
  without publicly exposing those ports.
- Long-running actions are tracked as operations with persisted step history.
- Backup and restore are part of the single-node recovery story.

---

## Non-goals

Vessel deliberately does not support: Kubernetes, multi-node orchestration, teams,
SaaS features, CI/CD pipelines, GitOps, cloud sync, or enterprise features.

---

## License

MIT
