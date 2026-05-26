package registry

// builtinTemplates returns the curated set of supported applications.
func builtinTemplates() []*AppTemplate {
	return []*AppTemplate{
		metabase(),
		n8n(),
		umami(),
		plausible(),
		openwebui(),
		plane(),
	}
}

func metabase() *AppTemplate {
	return &AppTemplate{
		ID:          "metabase",
		Name:        "Metabase",
		Description: "Open source business intelligence and analytics tool",
		Category:    "Analytics",
		Icon:        "📊",
		Version:     "latest",
		Image:       "metabase/metabase:latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3000, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "metabase-data", MountPath: "/metabase-data", Description: "Metabase application data"},
		},
		EnvVars: []EnvVar{
			{Key: "MB_DB_TYPE", Default: "h2", Description: "Database type: h2 (embedded) or postgres", Required: false},
			{Key: "MB_DB_FILE", Default: "/metabase-data/metabase.db", Description: "Database file path (h2 only)", Required: false},
			{Key: "MB_DB_HOST", Default: "metabase-db", Description: "PostgreSQL host (when MB_DB_TYPE=postgres)", Required: false},
			{Key: "MB_DB_PORT", Default: "5432", Description: "PostgreSQL port", Required: false},
			{Key: "MB_DB_DBNAME", Default: "metabase", Description: "PostgreSQL database name", Required: false},
			{Key: "MB_DB_USER", Default: "metabase", Description: "PostgreSQL username", Required: false},
			{Key: "MB_DB_PASS", Default: "metabase_password", Description: "PostgreSQL password", Required: false, Secret: true},
			{Key: "JAVA_TIMEZONE", Default: "UTC", Description: "JVM timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:3000/api/health"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  5,
		},
		ExtraServices: []ServiceDef{
			{
				Name:     "metabase-db",
				Image:    "postgres:15-alpine",
				Optional: true,
				Role:     "database",
				Environment: map[string]string{
					"POSTGRES_DB":       "metabase",
					"POSTGRES_USER":     "metabase",
					"POSTGRES_PASSWORD": "metabase_password",
				},
				Volumes: []Volume{
					{Name: "metabase-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U metabase"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
		},
	}
}

func n8n() *AppTemplate {
	return &AppTemplate{
		ID:          "n8n",
		Name:        "n8n",
		Description: "Workflow automation tool with a visual editor",
		Category:    "Automation",
		Icon:        "🔄",
		Version:     "latest",
		Image:       "n8nio/n8n:latest",
		ProxyPort:   5678,
		Ports: []Port{
			{Internal: 5678, External: 5678, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "n8n-data", MountPath: "/home/node/.n8n", Description: "n8n workflows and credentials"},
		},
		EnvVars: []EnvVar{
			{Key: "N8N_BASIC_AUTH_ACTIVE", Default: "true", Description: "Enable basic auth", Required: false},
			{Key: "N8N_BASIC_AUTH_USER", Default: "admin", Description: "Basic auth username", Required: false},
			{Key: "N8N_BASIC_AUTH_PASSWORD", Default: "", Description: "Basic auth password", Required: true, Secret: true},
			{Key: "N8N_HOST", Default: "0.0.0.0", Description: "Host to bind to", Required: false},
			{Key: "N8N_PORT", Default: "5678", Description: "Port to listen on", Required: false},
			{Key: "WEBHOOK_URL", Default: "", Description: "Public webhook URL (your domain)", Required: false},
			{Key: "GENERIC_TIMEZONE", Default: "UTC", Description: "Timezone for workflows", Required: false},
			{Key: "DB_TYPE", Default: "sqlite", Description: "Database type: sqlite or postgresdb", Required: false},
			{Key: "DB_POSTGRESDB_HOST", Default: "n8n-db", Description: "PostgreSQL host (when DB_TYPE=postgresdb)", Required: false},
			{Key: "DB_POSTGRESDB_PORT", Default: "5432", Description: "PostgreSQL port", Required: false},
			{Key: "DB_POSTGRESDB_DATABASE", Default: "n8n", Description: "PostgreSQL database name", Required: false},
			{Key: "DB_POSTGRESDB_USER", Default: "n8n", Description: "PostgreSQL username", Required: false},
			{Key: "DB_POSTGRESDB_PASSWORD", Default: "n8n_password", Description: "PostgreSQL password", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "wget", "--spider", "-q", "http://localhost:5678/healthz"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  3,
		},
		ExtraServices: []ServiceDef{
			{
				Name:     "n8n-db",
				Image:    "postgres:15-alpine",
				Optional: true,
				Role:     "database",
				Environment: map[string]string{
					"POSTGRES_DB":       "n8n",
					"POSTGRES_USER":     "n8n",
					"POSTGRES_PASSWORD": "n8n_password",
				},
				Volumes: []Volume{
					{Name: "n8n-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U n8n"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
		},
	}
}

func umami() *AppTemplate {
	return &AppTemplate{
		ID:          "umami",
		Name:        "Umami",
		Description: "Privacy-focused web analytics alternative to Google Analytics",
		Category:    "Analytics",
		Icon:        "📈",
		Version:     "latest",
		Image:       "ghcr.io/umami-software/umami:postgresql-latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3001, Protocol: "tcp"},
		},
		EnvVars: []EnvVar{
			{Key: "DATABASE_URL", Default: "postgresql://umami:umami_password@umami-db:5432/umami", Description: "PostgreSQL connection string", Required: true},
			{Key: "DATABASE_TYPE", Default: "postgresql", Description: "Database type", Required: false},
			{Key: "APP_SECRET", Default: "", Description: "Random secret for session encryption", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:3000/api/heartbeat"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  5,
		},
		ExtraServices: []ServiceDef{
			{
				Name:     "umami-db",
				Image:    "postgres:15-alpine",
				Optional: true,
				Role:     "database",
				Environment: map[string]string{
					"POSTGRES_DB":       "umami",
					"POSTGRES_USER":     "umami",
					"POSTGRES_PASSWORD": "umami_password",
				},
				Volumes: []Volume{
					{Name: "umami-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U umami"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
		},
	}
}

func plausible() *AppTemplate {
	return &AppTemplate{
		ID:          "plausible",
		Name:        "Plausible Analytics",
		Description: "Lightweight, privacy-friendly Google Analytics alternative",
		Category:    "Analytics",
		Icon:        "📉",
		Version:     "latest",
		Image:       "ghcr.io/plausible/community-edition:v2",
		ProxyPort:   8000,
		Ports: []Port{
			{Internal: 8000, External: 8000, Protocol: "tcp"},
		},
		EnvVars: []EnvVar{
			{Key: "BASE_URL", Default: "", Description: "Your public URL (e.g. https://analytics.example.com)", Required: true},
			{Key: "SECRET_KEY_BASE", Default: "", Description: "64-char random secret key", Required: true, Secret: true},
			{Key: "DATABASE_URL", Default: "postgres://plausible:plausible_password@plausible-db:5432/plausible", Description: "PostgreSQL connection string", Required: false},
			{Key: "CLICKHOUSE_DATABASE_URL", Default: "http://plausible-events-db:8123/plausible_events_db", Description: "ClickHouse connection string", Required: false},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "wget", "--spider", "-q", "http://localhost:8000/api/health"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  5,
		},
		ExtraServices: []ServiceDef{
			{
				Name:     "plausible-db",
				Image:    "postgres:14-alpine",
				Optional: true,
				Role:     "database",
				Environment: map[string]string{
					"POSTGRES_DB":       "plausible",
					"POSTGRES_USER":     "plausible",
					"POSTGRES_PASSWORD": "plausible_password",
				},
				Volumes: []Volume{
					{Name: "plausible-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U plausible"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
			{
				Name:     "plausible-events-db",
				Image:    "clickhouse/clickhouse-server:23.3-alpine",
				Optional: true,
				Role:     "events-database",
				Volumes: []Volume{
					{Name: "plausible-events-data", MountPath: "/var/lib/clickhouse", Description: "ClickHouse event data"},
				},
			},
		},
	}
}

func openwebui() *AppTemplate {
	return &AppTemplate{
		ID:          "open-webui",
		Name:        "Open WebUI",
		Description: "User-friendly web interface for Ollama and OpenAI-compatible APIs",
		Category:    "AI",
		Icon:        "🤖",
		Version:     "latest",
		Image:       "ghcr.io/open-webui/open-webui:main",
		ProxyPort:   8080,
		Ports: []Port{
			{Internal: 8080, External: 8080, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "open-webui-data", MountPath: "/app/backend/data", Description: "Open WebUI data and models"},
		},
		EnvVars: []EnvVar{
			{Key: "OLLAMA_BASE_URL", Default: "http://host.docker.internal:11434", Description: "Ollama API base URL", Required: false},
			{Key: "OPENAI_API_KEY", Default: "", Description: "OpenAI API key (optional)", Required: false, Secret: true},
			{Key: "WEBUI_SECRET_KEY", Default: "", Description: "Secret key for session security", Required: true, Secret: true},
			{Key: "WEBUI_AUTH", Default: "true", Description: "Enable authentication", Required: false},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:8080/health"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  3,
		},
	}
}

func plane() *AppTemplate {
	return &AppTemplate{
		ID:          "plane",
		Name:        "Plane",
		Description: "Open-source project management tool (Jira/Linear alternative)",
		Category:    "Productivity",
		Icon:        "✈️",
		Version:     "latest",
		Image:       "makeplane/plane-frontend:latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3002, Protocol: "tcp"},
		},
		EnvVars: []EnvVar{
			{Key: "NEXT_PUBLIC_API_BASE_URL", Default: "", Description: "Your public domain URL", Required: true},
			{Key: "SECRET_KEY", Default: "", Description: "Django secret key", Required: true, Secret: true},
			{Key: "DATABASE_URL", Default: "postgresql://plane:plane_password@plane-db:5432/plane", Description: "PostgreSQL connection string", Required: false},
			{Key: "REDIS_URL", Default: "redis://plane-redis:6379/", Description: "Redis connection string", Required: false},
			{Key: "AWS_S3_BUCKET_NAME", Default: "uploads", Description: "S3 bucket name for file uploads", Required: false},
			{Key: "FILE_SIZE_LIMIT", Default: "5242880", Description: "Max file upload size in bytes", Required: false},
		},
		HealthCheck: HealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:3000"},
			Interval: "30s",
			Timeout:  "10s",
			Retries:  5,
		},
		ExtraServices: []ServiceDef{
			{
				Name:     "plane-db",
				Image:    "postgres:15-alpine",
				Optional: true,
				Role:     "database",
				Environment: map[string]string{
					"POSTGRES_DB":       "plane",
					"POSTGRES_USER":     "plane",
					"POSTGRES_PASSWORD": "plane_password",
				},
				Volumes: []Volume{
					{Name: "plane-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD-SHELL", "pg_isready -U plane"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  5,
				},
			},
			{
				Name:     "plane-redis",
				Image:    "redis:7-alpine",
				Optional: true,
				Role:     "cache",
				Volumes: []Volume{
					{Name: "plane-redis-data", MountPath: "/data", Description: "Redis data"},
				},
				HealthCheck: HealthCheck{
					Test:     []string{"CMD", "redis-cli", "ping"},
					Interval: "10s",
					Timeout:  "5s",
					Retries:  3,
				},
			},
		},
	}
}
