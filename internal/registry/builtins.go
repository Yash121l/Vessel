package registry

// builtinTemplates returns the curated set of supported applications.
func builtinTemplates() []*AppTemplate {
	return []*AppTemplate{
		// Analytics
		metabase(),
		plausible(),
		umami(),
		superset(),
		redash(),

		// AI
		openwebui(),
		ollama(),

		// Automation
		n8n(),
		activepieces(),

		// CMS / No-code
		directus(),
		nocodb(),
		baserow(),

		// Communication
		mattermost(),
		rocketchat(),
		matrix(),
		jitsi(),
		zulip(),

		// Development
		gitea(),
		forgejo(),
		gitlab(),

		// DevOps
		portainer(),
		woodpecker(),
		drone(),

		// Email
		listmonk(),
		mailcow(),
		mailu(),

		// Home Automation
		homeassistant(),

		// Media
		jellyfin(),
		immich(),
		navidrome(),
		kavita(),
		plex(),

		// Monitoring
		grafana(),

		// Networking
		pihole(),
		nginxproxymanager(),
		traefik(),
		adguard(),
		wgeasy(),

		// Productivity
		plane(),
		vikunja(),
		taiga(),
		leantime(),
		appflowy(),

		// Security
		vaultwarden(),
		authelia(),
		keycloak(),
		wazuh(),
		crowdsec(),

		// Storage
		nextcloud(),
		seafile(),
		filebrowser(),
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

func grafana() *AppTemplate {
	return &AppTemplate{
		ID:          "grafana",
		Name:        "Grafana",
		Description: "Open source analytics and monitoring platform",
		Category:    "Monitoring",
		Icon:        "📡",
		Version:     "latest",
		Image:       "grafana/grafana:latest",
		ProxyPort:   3000,
		Ports:       []Port{{Internal: 3000, External: 3000, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "grafana-data", MountPath: "/var/lib/grafana", Description: "Grafana dashboards and configuration"},
		},
		EnvVars: []EnvVar{
			{Key: "GF_SECURITY_ADMIN_USER", Default: "admin", Description: "Admin username", Required: false},
			{Key: "GF_SECURITY_ADMIN_PASSWORD", Default: "", Description: "Admin password", Required: true, Secret: true},
			{Key: "GF_SERVER_ROOT_URL", Default: "", Description: "Public URL of your Grafana instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:3000/api/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func nextcloud() *AppTemplate {
	return &AppTemplate{
		ID:          "nextcloud",
		Name:        "Nextcloud",
		Description: "Self-hosted file sync, sharing, and collaboration platform",
		Category:    "Storage",
		Icon:        "☁️",
		Version:     "latest",
		Image:       "nextcloud:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "nextcloud-data", MountPath: "/var/www/html", Description: "Nextcloud application files and user data"},
		},
		EnvVars: []EnvVar{
			{Key: "NEXTCLOUD_ADMIN_USER", Default: "admin", Description: "Initial admin username", Required: false},
			{Key: "NEXTCLOUD_ADMIN_PASSWORD", Default: "", Description: "Initial admin password", Required: true, Secret: true},
			{Key: "NEXTCLOUD_TRUSTED_DOMAINS", Default: "", Description: "Space-separated trusted domains", Required: false},
			{Key: "POSTGRES_HOST", Default: "nextcloud-db", Description: "PostgreSQL host", Required: false},
			{Key: "POSTGRES_DB", Default: "nextcloud", Description: "PostgreSQL database name", Required: false},
			{Key: "POSTGRES_USER", Default: "nextcloud", Description: "PostgreSQL username", Required: false},
			{Key: "POSTGRES_PASSWORD", Default: "nextcloud_password", Description: "PostgreSQL password", Required: false, Secret: true},
			{Key: "REDIS_HOST", Default: "nextcloud-redis", Description: "Redis host for caching", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/status.php"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "nextcloud-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "nextcloud", "POSTGRES_USER": "nextcloud", "POSTGRES_PASSWORD": "nextcloud_password"},
				Volumes:     []Volume{{Name: "nextcloud-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U nextcloud"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "nextcloud-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "nextcloud-redis-data", MountPath: "/data", Description: "Redis cache data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func vaultwarden() *AppTemplate {
	return &AppTemplate{
		ID:          "vaultwarden",
		Name:        "Vaultwarden",
		Description: "Lightweight Bitwarden-compatible password manager server",
		Category:    "Security",
		Icon:        "🔐",
		Version:     "latest",
		Image:       "vaultwarden/server:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "vaultwarden-data", MountPath: "/data", Description: "Vaultwarden vault data and attachments"},
		},
		EnvVars: []EnvVar{
			{Key: "ADMIN_TOKEN", Default: "", Description: "Token for the /admin panel (leave empty to disable)", Required: false, Secret: true},
			{Key: "DOMAIN", Default: "", Description: "Full public URL (e.g. https://vault.example.com)", Required: false},
			{Key: "SIGNUPS_ALLOWED", Default: "true", Description: "Allow new user registrations", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/alive"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func jellyfin() *AppTemplate {
	return &AppTemplate{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "Free and open-source media server",
		Category:    "Media",
		Icon:        "🎬",
		Version:     "latest",
		Image:       "jellyfin/jellyfin:latest",
		ProxyPort:   8096,
		Ports: []Port{
			{Internal: 8096, External: 8096, Protocol: "tcp"},
			{Internal: 8920, External: 8920, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "jellyfin-config", MountPath: "/config", Description: "Jellyfin configuration and metadata"},
			{Name: "jellyfin-cache", MountPath: "/cache", Description: "Jellyfin transcoding cache"},
			{Name: "jellyfin-media", MountPath: "/media", Description: "Media library"},
		},
		EnvVars: []EnvVar{
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8096/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func homeassistant() *AppTemplate {
	return &AppTemplate{
		ID:          "homeassistant",
		Name:        "Home Assistant",
		Description: "Open-source home automation platform",
		Category:    "Home Automation",
		Icon:        "🏠",
		Version:     "latest",
		Image:       "ghcr.io/home-assistant/home-assistant:stable",
		ProxyPort:   8123,
		Ports:       []Port{{Internal: 8123, External: 8123, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "homeassistant-config", MountPath: "/config", Description: "Home Assistant configuration and automations"},
		},
		EnvVars: []EnvVar{
			{Key: "TZ", Default: "UTC", Description: "Timezone (e.g. America/New_York)", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8123"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
	}
}

func pihole() *AppTemplate {
	return &AppTemplate{
		ID:          "pihole",
		Name:        "Pi-hole",
		Description: "Network-wide ad blocker and DNS sinkhole",
		Category:    "Networking",
		Icon:        "🕳️",
		Version:     "latest",
		Image:       "pihole/pihole:latest",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 53, External: 53, Protocol: "tcp"},
			{Internal: 53, External: 53, Protocol: "udp"},
		},
		Volumes: []Volume{
			{Name: "pihole-etc", MountPath: "/etc/pihole", Description: "Pi-hole configuration and blocklists"},
			{Name: "pihole-dnsmasq", MountPath: "/etc/dnsmasq.d", Description: "dnsmasq configuration"},
		},
		EnvVars: []EnvVar{
			{Key: "WEBPASSWORD", Default: "", Description: "Password for the Pi-hole web admin panel", Required: true, Secret: true},
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
			{Key: "PIHOLE_DNS_", Default: "8.8.8.8;8.8.4.4", Description: "Upstream DNS servers (semicolon-separated)", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/admin"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func portainer() *AppTemplate {
	return &AppTemplate{
		ID:          "portainer",
		Name:        "Portainer",
		Description: "Container management UI for Docker and Kubernetes",
		Category:    "DevOps",
		Icon:        "🐳",
		Version:     "latest",
		Image:       "portainer/portainer-ce:latest",
		ProxyPort:   9000,
		Ports: []Port{
			{Internal: 9000, External: 9000, Protocol: "tcp"},
			{Internal: 9443, External: 9443, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "portainer-data", MountPath: "/data", Description: "Portainer configuration and state"},
			{Name: "docker-sock", MountPath: "/var/run/docker.sock", Description: "Docker socket"},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:9000"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func gitea() *AppTemplate {
	return &AppTemplate{
		ID:          "gitea",
		Name:        "Gitea",
		Description: "Lightweight self-hosted Git service",
		Category:    "Development",
		Icon:        "🍵",
		Version:     "latest",
		Image:       "gitea/gitea:latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3000, Protocol: "tcp"},
			{Internal: 22, External: 2222, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "gitea-data", MountPath: "/data", Description: "Gitea repositories, configuration, and data"},
		},
		EnvVars: []EnvVar{
			{Key: "GITEA__database__DB_TYPE", Default: "sqlite3", Description: "Database type: sqlite3, postgres, or mysql", Required: false},
			{Key: "GITEA__database__HOST", Default: "gitea-db:5432", Description: "Database host", Required: false},
			{Key: "GITEA__database__NAME", Default: "gitea", Description: "Database name", Required: false},
			{Key: "GITEA__database__USER", Default: "gitea", Description: "Database username", Required: false},
			{Key: "GITEA__database__PASSWD", Default: "gitea_password", Description: "Database password", Required: false, Secret: true},
			{Key: "GITEA__server__DOMAIN", Default: "", Description: "Domain name for Gitea", Required: false},
			{Key: "GITEA__server__ROOT_URL", Default: "", Description: "Full public URL", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:3000"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "gitea-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "gitea", "POSTGRES_USER": "gitea", "POSTGRES_PASSWORD": "gitea_password"},
				Volumes:     []Volume{{Name: "gitea-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U gitea"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func gitlab() *AppTemplate {
	return &AppTemplate{
		ID:          "gitlab",
		Name:        "GitLab CE",
		Description: "Full-featured self-hosted Git platform with CI/CD",
		Category:    "Development",
		Icon:        "🦊",
		Version:     "latest",
		Image:       "gitlab/gitlab-ce:latest",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 443, External: 8443, Protocol: "tcp"},
			{Internal: 22, External: 2222, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "gitlab-config", MountPath: "/etc/gitlab", Description: "GitLab configuration"},
			{Name: "gitlab-logs", MountPath: "/var/log/gitlab", Description: "GitLab logs"},
			{Name: "gitlab-data", MountPath: "/var/opt/gitlab", Description: "GitLab repositories and data"},
		},
		EnvVars: []EnvVar{
			{Key: "GITLAB_ROOT_PASSWORD", Default: "", Description: "Initial root password", Required: true, Secret: true},
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/-/health"}, Interval: "60s", Timeout: "30s", Retries: 10,
		},
	}
}

func immich() *AppTemplate {
	return &AppTemplate{
		ID:          "immich",
		Name:        "Immich",
		Description: "High-performance self-hosted photo and video backup solution",
		Category:    "Media",
		Icon:        "📷",
		Version:     "latest",
		Image:       "ghcr.io/immich-app/immich-server:release",
		ProxyPort:   2283,
		Ports:       []Port{{Internal: 2283, External: 2283, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "immich-upload", MountPath: "/usr/src/app/upload", Description: "Photo and video uploads"},
		},
		EnvVars: []EnvVar{
			{Key: "DB_HOSTNAME", Default: "immich-db", Description: "PostgreSQL host", Required: false},
			{Key: "DB_DATABASE_NAME", Default: "immich", Description: "PostgreSQL database name", Required: false},
			{Key: "DB_USERNAME", Default: "immich", Description: "PostgreSQL username", Required: false},
			{Key: "DB_PASSWORD", Default: "immich_password", Description: "PostgreSQL password", Required: false, Secret: true},
			{Key: "REDIS_HOSTNAME", Default: "immich-redis", Description: "Redis host", Required: false},
			{Key: "JWT_SECRET", Default: "", Description: "Secret for JWT token signing", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:2283/api/server-info/ping"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "immich-db", Image: "tensorchord/pgvecto-rs:pg14-v0.2.0", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "immich", "POSTGRES_USER": "immich", "POSTGRES_PASSWORD": "immich_password"},
				Volumes:     []Volume{{Name: "immich-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U immich"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "immich-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
			{
				Name: "immich-machine-learning", Image: "ghcr.io/immich-app/immich-machine-learning:release", Optional: true, Role: "ml",
				Volumes: []Volume{{Name: "immich-model-cache", MountPath: "/cache", Description: "Machine learning model cache"}},
			},
		},
	}
}

func nocodb() *AppTemplate {
	return &AppTemplate{
		ID:          "nocodb",
		Name:        "NocoDB",
		Description: "Open-source Airtable alternative — turn any database into a smart spreadsheet",
		Category:    "Productivity",
		Icon:        "📋",
		Version:     "latest",
		Image:       "nocodb/nocodb:latest",
		ProxyPort:   8080,
		Ports:       []Port{{Internal: 8080, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "nocodb-data", MountPath: "/usr/app/data", Description: "NocoDB metadata and SQLite database"},
		},
		EnvVars: []EnvVar{
			{Key: "NC_AUTH_JWT_SECRET", Default: "", Description: "JWT secret for authentication", Required: true, Secret: true},
			{Key: "NC_DB", Default: "", Description: "External database URL (leave empty for built-in SQLite)", Required: false},
			{Key: "NC_PUBLIC_URL", Default: "", Description: "Public URL of your NocoDB instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8080/api/v1/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func baserow() *AppTemplate {
	return &AppTemplate{
		ID:          "baserow",
		Name:        "Baserow",
		Description: "Open-source no-code database and Airtable alternative",
		Category:    "Productivity",
		Icon:        "🗃️",
		Version:     "latest",
		Image:       "baserow/baserow:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "baserow-data", MountPath: "/baserow/data", Description: "Baserow application data"},
		},
		EnvVars: []EnvVar{
			{Key: "BASEROW_PUBLIC_URL", Default: "", Description: "Public URL of your Baserow instance", Required: true},
			{Key: "SECRET_KEY", Default: "", Description: "Django secret key", Required: true, Secret: true},
			{Key: "DATABASE_URL", Default: "postgresql://baserow:baserow_password@baserow-db:5432/baserow", Description: "PostgreSQL connection string", Required: false},
			{Key: "REDIS_URL", Default: "redis://baserow-redis:6379/0", Description: "Redis connection string", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/api/_health/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "baserow-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "baserow", "POSTGRES_USER": "baserow", "POSTGRES_PASSWORD": "baserow_password"},
				Volumes:     []Volume{{Name: "baserow-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U baserow"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "baserow-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "baserow-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func mattermost() *AppTemplate {
	return &AppTemplate{
		ID:          "mattermost",
		Name:        "Mattermost",
		Description: "Open-source team messaging and collaboration platform",
		Category:    "Communication",
		Icon:        "💬",
		Version:     "latest",
		Image:       "mattermost/mattermost-team-edition:latest",
		ProxyPort:   8065,
		Ports:       []Port{{Internal: 8065, External: 8065, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "mattermost-config", MountPath: "/mattermost/config", Description: "Mattermost configuration"},
			{Name: "mattermost-data", MountPath: "/mattermost/data", Description: "Mattermost user data and attachments"},
			{Name: "mattermost-plugins", MountPath: "/mattermost/plugins", Description: "Mattermost plugins"},
		},
		EnvVars: []EnvVar{
			{Key: "MM_SQLSETTINGS_DRIVERNAME", Default: "postgres", Description: "Database driver", Required: false},
			{Key: "MM_SQLSETTINGS_DATASOURCE", Default: "postgres://mattermost:mattermost_password@mattermost-db:5432/mattermost?sslmode=disable", Description: "Database connection string", Required: false},
			{Key: "MM_SERVICESETTINGS_SITEURL", Default: "", Description: "Public URL of your Mattermost instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8065/api/v4/system/ping"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "mattermost-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "mattermost", "POSTGRES_USER": "mattermost", "POSTGRES_PASSWORD": "mattermost_password"},
				Volumes:     []Volume{{Name: "mattermost-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U mattermost"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func rocketchat() *AppTemplate {
	return &AppTemplate{
		ID:          "rocketchat",
		Name:        "Rocket.Chat",
		Description: "Open-source team communication and messaging platform",
		Category:    "Communication",
		Icon:        "🚀",
		Version:     "latest",
		Image:       "rocket.chat:latest",
		ProxyPort:   3000,
		Ports:       []Port{{Internal: 3000, External: 3000, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "rocketchat-uploads", MountPath: "/app/uploads", Description: "Rocket.Chat file uploads"},
		},
		EnvVars: []EnvVar{
			{Key: "ROOT_URL", Default: "", Description: "Public URL of your Rocket.Chat instance", Required: true},
			{Key: "MONGO_URL", Default: "mongodb://rocketchat-db:27017/rocketchat", Description: "MongoDB connection string", Required: false},
			{Key: "MONGO_OPLOG_URL", Default: "mongodb://rocketchat-db:27017/local", Description: "MongoDB oplog URL", Required: false},
			{Key: "ADMIN_PASS", Default: "", Description: "Initial admin password", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:3000/api/v1/info"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "rocketchat-db", Image: "mongo:6", Optional: true, Role: "database",
				Environment: map[string]string{"MONGO_INITDB_DATABASE": "rocketchat"},
				Volumes:     []Volume{{Name: "rocketchat-db-data", MountPath: "/data/db", Description: "MongoDB data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "mongosh", "--eval", "db.adminCommand('ping')"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func matrix() *AppTemplate {
	return &AppTemplate{
		ID:          "matrix",
		Name:        "Matrix / Element (Synapse)",
		Description: "Decentralized, federated messaging with Element web client",
		Category:    "Communication",
		Icon:        "🔷",
		Version:     "latest",
		Image:       "matrixdotorg/synapse:latest",
		ProxyPort:   8008,
		Ports:       []Port{{Internal: 8008, External: 8008, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "synapse-data", MountPath: "/data", Description: "Synapse homeserver data and media"},
		},
		EnvVars: []EnvVar{
			{Key: "SYNAPSE_SERVER_NAME", Default: "", Description: "Your Matrix server name (e.g. matrix.example.com)", Required: true},
			{Key: "SYNAPSE_REPORT_STATS", Default: "no", Description: "Report anonymous usage statistics", Required: false},
			{Key: "POSTGRES_HOST", Default: "synapse-db", Description: "PostgreSQL host", Required: false},
			{Key: "POSTGRES_DB", Default: "synapse", Description: "PostgreSQL database name", Required: false},
			{Key: "POSTGRES_USER", Default: "synapse", Description: "PostgreSQL username", Required: false},
			{Key: "POSTGRES_PASSWORD", Default: "synapse_password", Description: "PostgreSQL password", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8008/health"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "synapse-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{
					"POSTGRES_DB": "synapse", "POSTGRES_USER": "synapse", "POSTGRES_PASSWORD": "synapse_password",
					"POSTGRES_INITDB_ARGS": "--encoding=UTF-8 --lc-collate=C --lc-ctype=C",
				},
				Volumes:     []Volume{{Name: "synapse-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U synapse"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "element-web", Image: "vectorim/element-web:latest", Optional: true, Role: "web-client",
			},
		},
	}
}

func jitsi() *AppTemplate {
	return &AppTemplate{
		ID:          "jitsi",
		Name:        "Jitsi Meet",
		Description: "Open-source video conferencing solution",
		Category:    "Communication",
		Icon:        "📹",
		Version:     "latest",
		Image:       "jitsi/web:latest",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 10000, External: 10000, Protocol: "udp"},
		},
		Volumes: []Volume{
			{Name: "jitsi-web-config", MountPath: "/config", Description: "Jitsi web configuration"},
		},
		EnvVars: []EnvVar{
			{Key: "PUBLIC_URL", Default: "", Description: "Public URL of your Jitsi instance", Required: true},
			{Key: "JICOFO_AUTH_PASSWORD", Default: "", Description: "Jicofo authentication password", Required: true, Secret: true},
			{Key: "JVB_AUTH_PASSWORD", Default: "", Description: "JVB authentication password", Required: true, Secret: true},
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{Name: "jitsi-prosody", Image: "jitsi/prosody:latest", Optional: false, Role: "xmpp"},
			{Name: "jitsi-jicofo", Image: "jitsi/jicofo:latest", Optional: false, Role: "focus"},
			{Name: "jitsi-jvb", Image: "jitsi/jvb:latest", Optional: false, Role: "videobridge"},
		},
	}
}

func zulip() *AppTemplate {
	return &AppTemplate{
		ID:          "zulip",
		Name:        "Zulip",
		Description: "Powerful open-source team chat with topic-based threading",
		Category:    "Communication",
		Icon:        "💭",
		Version:     "latest",
		Image:       "zulip/docker-zulip:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "zulip-data", MountPath: "/data", Description: "Zulip application data and uploads"},
		},
		EnvVars: []EnvVar{
			{Key: "SETTING_EXTERNAL_HOST", Default: "", Description: "Public hostname (e.g. zulip.example.com)", Required: true},
			{Key: "SETTING_ZULIP_ADMINISTRATOR", Default: "", Description: "Admin email address", Required: true},
			{Key: "SECRETS_secret_key", Default: "", Description: "Django secret key", Required: true, Secret: true},
			{Key: "POSTGRES_HOST", Default: "zulip-db", Description: "PostgreSQL host", Required: false},
			{Key: "POSTGRES_PASSWORD", Default: "zulip_password", Description: "PostgreSQL password", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "60s", Timeout: "30s", Retries: 10,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "zulip-db", Image: "zulip/zulip-postgresql:14", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "zulip", "POSTGRES_USER": "zulip", "POSTGRES_PASSWORD": "zulip_password"},
				Volumes:     []Volume{{Name: "zulip-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U zulip"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "zulip-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "zulip-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
			{
				Name: "zulip-rabbitmq", Image: "rabbitmq:3.12-management-alpine", Optional: true, Role: "queue",
				Volumes: []Volume{{Name: "zulip-rabbitmq-data", MountPath: "/var/lib/rabbitmq", Description: "RabbitMQ data"}},
			},
		},
	}
}

func mailcow() *AppTemplate {
	return &AppTemplate{
		ID:          "mailcow",
		Name:        "Mailcow",
		Description: "Full-featured self-hosted email server suite (SMTP, IMAP, webmail)",
		Category:    "Email",
		Icon:        "📧",
		Version:     "latest",
		Image:       "mailcow/mailcow-dockerized:latest",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 25, External: 25, Protocol: "tcp"},
			{Internal: 587, External: 587, Protocol: "tcp"},
			{Internal: 993, External: 993, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "mailcow-vmail", MountPath: "/var/vmail", Description: "Mail storage"},
		},
		EnvVars: []EnvVar{
			{Key: "MAILCOW_HOSTNAME", Default: "", Description: "Primary mail hostname (e.g. mail.example.com)", Required: true},
			{Key: "DBPASS", Default: "", Description: "MySQL database password", Required: true, Secret: true},
			{Key: "DBROOT", Default: "", Description: "MySQL root password", Required: true, Secret: true},
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "60s", Timeout: "30s", Retries: 10,
		},
	}
}

func listmonk() *AppTemplate {
	return &AppTemplate{
		ID:          "listmonk",
		Name:        "Listmonk",
		Description: "High-performance self-hosted newsletter and mailing list manager",
		Category:    "Email",
		Icon:        "📨",
		Version:     "latest",
		Image:       "listmonk/listmonk:latest",
		ProxyPort:   9000,
		Ports:       []Port{{Internal: 9000, External: 9000, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "listmonk-uploads", MountPath: "/listmonk/uploads", Description: "Listmonk uploaded media"},
		},
		EnvVars: []EnvVar{
			{Key: "LISTMONK_db__host", Default: "listmonk-db", Description: "PostgreSQL host", Required: false},
			{Key: "LISTMONK_db__port", Default: "5432", Description: "PostgreSQL port", Required: false},
			{Key: "LISTMONK_db__user", Default: "listmonk", Description: "PostgreSQL username", Required: false},
			{Key: "LISTMONK_db__password", Default: "listmonk_password", Description: "PostgreSQL password", Required: false, Secret: true},
			{Key: "LISTMONK_db__database", Default: "listmonk", Description: "PostgreSQL database name", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:9000"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "listmonk-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "listmonk", "POSTGRES_USER": "listmonk", "POSTGRES_PASSWORD": "listmonk_password"},
				Volumes:     []Volume{{Name: "listmonk-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U listmonk"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func nginxproxymanager() *AppTemplate {
	return &AppTemplate{
		ID:          "nginx-proxy-manager",
		Name:        "Nginx Proxy Manager",
		Description: "Easy reverse proxy management with SSL via Let's Encrypt",
		Category:    "Networking",
		Icon:        "🔀",
		Version:     "latest",
		Image:       "jc21/nginx-proxy-manager:latest",
		ProxyPort:   81,
		Ports: []Port{
			{Internal: 80, External: 80, Protocol: "tcp"},
			{Internal: 443, External: 443, Protocol: "tcp"},
			{Internal: 81, External: 81, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "npm-data", MountPath: "/data", Description: "Nginx Proxy Manager configuration and certificates"},
			{Name: "npm-letsencrypt", MountPath: "/etc/letsencrypt", Description: "Let's Encrypt certificates"},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:81"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func traefik() *AppTemplate {
	return &AppTemplate{
		ID:          "traefik",
		Name:        "Traefik",
		Description: "Modern reverse proxy and load balancer with automatic SSL",
		Category:    "Networking",
		Icon:        "🔁",
		Version:     "latest",
		Image:       "traefik:latest",
		ProxyPort:   8080,
		Ports: []Port{
			{Internal: 80, External: 80, Protocol: "tcp"},
			{Internal: 443, External: 443, Protocol: "tcp"},
			{Internal: 8080, External: 8080, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "traefik-config", MountPath: "/etc/traefik", Description: "Traefik configuration"},
			{Name: "traefik-certs", MountPath: "/certs", Description: "TLS certificate storage"},
			{Name: "docker-sock", MountPath: "/var/run/docker.sock", Description: "Docker socket for service discovery"},
		},
		EnvVars: []EnvVar{
			{Key: "TRAEFIK_API_INSECURE", Default: "true", Description: "Enable insecure API/dashboard", Required: false},
			{Key: "TRAEFIK_PROVIDERS_DOCKER", Default: "true", Description: "Enable Docker provider", Required: false},
			{Key: "TRAEFIK_CERTIFICATESRESOLVERS_LETSENCRYPT_ACME_EMAIL", Default: "", Description: "Email for Let's Encrypt notifications", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "traefik", "healthcheck"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func adguard() *AppTemplate {
	return &AppTemplate{
		ID:          "adguard",
		Name:        "AdGuard Home",
		Description: "Network-wide DNS-based ad and tracker blocker",
		Category:    "Networking",
		Icon:        "🛡️",
		Version:     "latest",
		Image:       "adguard/adguardhome:latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3000, Protocol: "tcp"},
			{Internal: 53, External: 53, Protocol: "tcp"},
			{Internal: 53, External: 53, Protocol: "udp"},
			{Internal: 80, External: 8080, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "adguard-work", MountPath: "/opt/adguardhome/work", Description: "AdGuard Home working data and query logs"},
			{Name: "adguard-conf", MountPath: "/opt/adguardhome/conf", Description: "AdGuard Home configuration"},
		},
		EnvVars: []EnvVar{
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:3000"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func wgeasy() *AppTemplate {
	return &AppTemplate{
		ID:          "wgeasy",
		Name:        "WireGuard (WG-Easy)",
		Description: "Easy-to-use WireGuard VPN server with web UI",
		Category:    "Networking",
		Icon:        "🔒",
		Version:     "latest",
		Image:       "ghcr.io/wg-easy/wg-easy:latest",
		ProxyPort:   51821,
		Ports: []Port{
			{Internal: 51820, External: 51820, Protocol: "udp"},
			{Internal: 51821, External: 51821, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "wgeasy-data", MountPath: "/etc/wireguard", Description: "WireGuard configuration and peer keys"},
		},
		EnvVars: []EnvVar{
			{Key: "WG_HOST", Default: "", Description: "Public hostname or IP of your server", Required: true},
			{Key: "PASSWORD", Default: "", Description: "Web UI admin password", Required: true, Secret: true},
			{Key: "WG_DEFAULT_DNS", Default: "1.1.1.1", Description: "DNS server for VPN clients", Required: false},
			{Key: "WG_ALLOWED_IPS", Default: "0.0.0.0/0, ::/0", Description: "Allowed IPs for VPN clients", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:51821"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func authelia() *AppTemplate {
	return &AppTemplate{
		ID:          "authelia",
		Name:        "Authelia",
		Description: "Open-source authentication and authorization server with SSO and 2FA",
		Category:    "Security",
		Icon:        "🔑",
		Version:     "latest",
		Image:       "authelia/authelia:latest",
		ProxyPort:   9091,
		Ports:       []Port{{Internal: 9091, External: 9091, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "authelia-config", MountPath: "/config", Description: "Authelia configuration and user database"},
		},
		EnvVars: []EnvVar{
			{Key: "AUTHELIA_JWT_SECRET", Default: "", Description: "JWT secret for session tokens", Required: true, Secret: true},
			{Key: "AUTHELIA_SESSION_SECRET", Default: "", Description: "Session encryption secret", Required: true, Secret: true},
			{Key: "AUTHELIA_STORAGE_ENCRYPTION_KEY", Default: "", Description: "Storage encryption key (min 20 chars)", Required: true, Secret: true},
			{Key: "AUTHELIA_STORAGE_POSTGRES_HOST", Default: "authelia-db", Description: "PostgreSQL host", Required: false},
			{Key: "AUTHELIA_STORAGE_POSTGRES_DATABASE", Default: "authelia", Description: "PostgreSQL database name", Required: false},
			{Key: "AUTHELIA_STORAGE_POSTGRES_USERNAME", Default: "authelia", Description: "PostgreSQL username", Required: false},
			{Key: "AUTHELIA_STORAGE_POSTGRES_PASSWORD", Default: "authelia_password", Description: "PostgreSQL password", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:9091/api/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "authelia-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "authelia", "POSTGRES_USER": "authelia", "POSTGRES_PASSWORD": "authelia_password"},
				Volumes:     []Volume{{Name: "authelia-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U authelia"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "authelia-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "authelia-redis-data", MountPath: "/data", Description: "Redis session data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func keycloak() *AppTemplate {
	return &AppTemplate{
		ID:          "keycloak",
		Name:        "Keycloak",
		Description: "Enterprise-grade open-source identity and access management with SSO",
		Category:    "Security",
		Icon:        "🗝️",
		Version:     "latest",
		Image:       "quay.io/keycloak/keycloak:latest",
		ProxyPort:   8080,
		Ports:       []Port{{Internal: 8080, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "keycloak-data", MountPath: "/opt/keycloak/data", Description: "Keycloak realm and user data"},
		},
		EnvVars: []EnvVar{
			{Key: "KEYCLOAK_ADMIN", Default: "admin", Description: "Initial admin username", Required: false},
			{Key: "KEYCLOAK_ADMIN_PASSWORD", Default: "", Description: "Initial admin password", Required: true, Secret: true},
			{Key: "KC_DB", Default: "postgres", Description: "Database type: postgres, mysql, or dev-file", Required: false},
			{Key: "KC_DB_URL", Default: "jdbc:postgresql://keycloak-db:5432/keycloak", Description: "JDBC database URL", Required: false},
			{Key: "KC_DB_USERNAME", Default: "keycloak", Description: "Database username", Required: false},
			{Key: "KC_DB_PASSWORD", Default: "keycloak_password", Description: "Database password", Required: false, Secret: true},
			{Key: "KC_HOSTNAME", Default: "", Description: "Public hostname (e.g. auth.example.com)", Required: false},
			{Key: "KC_PROXY", Default: "edge", Description: "Proxy mode (edge, reencrypt, passthrough)", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8080/health/ready"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "keycloak-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "keycloak", "POSTGRES_USER": "keycloak", "POSTGRES_PASSWORD": "keycloak_password"},
				Volumes:     []Volume{{Name: "keycloak-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U keycloak"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func wazuh() *AppTemplate {
	return &AppTemplate{
		ID:          "wazuh",
		Name:        "Wazuh",
		Description: "Open-source security platform for threat detection and incident response",
		Category:    "Security",
		Icon:        "🛡️",
		Version:     "latest",
		Image:       "wazuh/wazuh-manager:latest",
		ProxyPort:   5601,
		Ports: []Port{
			{Internal: 1514, External: 1514, Protocol: "tcp"},
			{Internal: 1515, External: 1515, Protocol: "tcp"},
			{Internal: 55000, External: 55000, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "wazuh-manager-config", MountPath: "/var/ossec/etc", Description: "Wazuh manager configuration"},
			{Name: "wazuh-manager-logs", MountPath: "/var/ossec/logs", Description: "Wazuh manager logs"},
		},
		EnvVars: []EnvVar{
			{Key: "API_PASSWORD", Default: "", Description: "Wazuh API password", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:55000/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "wazuh-indexer", Image: "wazuh/wazuh-indexer:latest", Optional: true, Role: "indexer",
				Volumes: []Volume{{Name: "wazuh-indexer-data", MountPath: "/var/lib/wazuh-indexer", Description: "Wazuh indexer data"}},
			},
			{Name: "wazuh-dashboard", Image: "wazuh/wazuh-dashboard:latest", Optional: true, Role: "dashboard"},
		},
	}
}

func crowdsec() *AppTemplate {
	return &AppTemplate{
		ID:          "crowdsec",
		Name:        "CrowdSec",
		Description: "Collaborative, open-source security engine for threat detection and blocking",
		Category:    "Security",
		Icon:        "🚨",
		Version:     "latest",
		Image:       "crowdsecurity/crowdsec:latest",
		ProxyPort:   8080,
		Ports:       []Port{{Internal: 8080, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "crowdsec-config", MountPath: "/etc/crowdsec", Description: "CrowdSec configuration and scenarios"},
			{Name: "crowdsec-data", MountPath: "/var/lib/crowdsec/data", Description: "CrowdSec database and decisions"},
		},
		EnvVars: []EnvVar{
			{Key: "COLLECTIONS", Default: "crowdsecurity/linux crowdsecurity/nginx", Description: "CrowdSec collections to install", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "cscli", "version"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func superset() *AppTemplate {
	return &AppTemplate{
		ID:          "superset",
		Name:        "Apache Superset",
		Description: "Modern open-source data exploration and visualization platform",
		Category:    "Analytics",
		Icon:        "📊",
		Version:     "latest",
		Image:       "apache/superset:latest",
		ProxyPort:   8088,
		Ports:       []Port{{Internal: 8088, External: 8088, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "superset-home", MountPath: "/app/superset_home", Description: "Superset configuration and database"},
		},
		EnvVars: []EnvVar{
			{Key: "SUPERSET_SECRET_KEY", Default: "", Description: "Secret key for session security", Required: true, Secret: true},
			{Key: "DATABASE_URL", Default: "postgresql+psycopg2://superset:superset_password@superset-db:5432/superset", Description: "SQLAlchemy database URL", Required: false},
			{Key: "REDIS_URL", Default: "redis://superset-redis:6379/0", Description: "Redis URL for caching", Required: false},
			{Key: "ADMIN_USERNAME", Default: "admin", Description: "Initial admin username", Required: false},
			{Key: "ADMIN_PASSWORD", Default: "", Description: "Initial admin password", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8088/health"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "superset-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "superset", "POSTGRES_USER": "superset", "POSTGRES_PASSWORD": "superset_password"},
				Volumes:     []Volume{{Name: "superset-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U superset"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "superset-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "superset-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func redash() *AppTemplate {
	return &AppTemplate{
		ID:          "redash",
		Name:        "Redash",
		Description: "Open-source data visualization and dashboarding tool",
		Category:    "Analytics",
		Icon:        "📈",
		Version:     "latest",
		Image:       "redash/redash:latest",
		ProxyPort:   5000,
		Ports:       []Port{{Internal: 5000, External: 5000, Protocol: "tcp"}},
		EnvVars: []EnvVar{
			{Key: "REDASH_SECRET_KEY", Default: "", Description: "Secret key for session security", Required: true, Secret: true},
			{Key: "REDASH_DATABASE_URL", Default: "postgresql://redash:redash_password@redash-db/redash", Description: "PostgreSQL connection string", Required: false},
			{Key: "REDASH_REDIS_URL", Default: "redis://redash-redis:6379/0", Description: "Redis connection string", Required: false},
			{Key: "REDASH_COOKIE_SECRET", Default: "", Description: "Cookie secret key", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:5000/ping"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "redash-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "redash", "POSTGRES_USER": "redash", "POSTGRES_PASSWORD": "redash_password"},
				Volumes:     []Volume{{Name: "redash-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U redash"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "redash-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "redash-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func directus() *AppTemplate {
	return &AppTemplate{
		ID:          "directus",
		Name:        "Directus",
		Description: "Open-source headless CMS and data platform",
		Category:    "CMS",
		Icon:        "🗂️",
		Version:     "latest",
		Image:       "directus/directus:latest",
		ProxyPort:   8055,
		Ports:       []Port{{Internal: 8055, External: 8055, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "directus-uploads", MountPath: "/directus/uploads", Description: "Directus file uploads"},
			{Name: "directus-extensions", MountPath: "/directus/extensions", Description: "Directus custom extensions"},
		},
		EnvVars: []EnvVar{
			{Key: "SECRET", Default: "", Description: "Secret key for JWT and hashing", Required: true, Secret: true},
			{Key: "ADMIN_EMAIL", Default: "admin@directus.local", Description: "Initial admin email", Required: false},
			{Key: "ADMIN_PASSWORD", Default: "", Description: "Initial admin password", Required: true, Secret: true},
			{Key: "DB_CLIENT", Default: "pg", Description: "Database client: pg, mysql, sqlite3", Required: false},
			{Key: "DB_HOST", Default: "directus-db", Description: "Database host", Required: false},
			{Key: "DB_DATABASE", Default: "directus", Description: "Database name", Required: false},
			{Key: "DB_USER", Default: "directus", Description: "Database username", Required: false},
			{Key: "DB_PASSWORD", Default: "directus_password", Description: "Database password", Required: false, Secret: true},
			{Key: "REDIS", Default: "redis://directus-redis:6379", Description: "Redis connection string", Required: false},
			{Key: "PUBLIC_URL", Default: "", Description: "Public URL of your Directus instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:8055/server/health"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "directus-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "directus", "POSTGRES_USER": "directus", "POSTGRES_PASSWORD": "directus_password"},
				Volumes:     []Volume{{Name: "directus-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U directus"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "directus-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "directus-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func appflowy() *AppTemplate {
	return &AppTemplate{
		ID:          "appflowy",
		Name:        "AppFlowy",
		Description: "Open-source Notion alternative for notes, wikis, and project management",
		Category:    "Productivity",
		Icon:        "📝",
		Version:     "latest",
		Image:       "appflowyinc/appflowy-cloud:latest",
		ProxyPort:   8000,
		Ports:       []Port{{Internal: 8000, External: 8000, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "appflowy-data", MountPath: "/var/lib/appflowy", Description: "AppFlowy application data"},
		},
		EnvVars: []EnvVar{
			{Key: "APPFLOWY_DATABASE_URL", Default: "postgres://appflowy:appflowy_password@appflowy-db:5432/appflowy", Description: "PostgreSQL connection string", Required: false},
			{Key: "APPFLOWY_REDIS_URI", Default: "redis://appflowy-redis:6379", Description: "Redis connection string", Required: false},
			{Key: "APPFLOWY_GOTRUE_JWT_SECRET", Default: "", Description: "JWT secret for authentication", Required: true, Secret: true},
			{Key: "APPFLOWY_GOTRUE_ADMIN_EMAIL", Default: "admin@appflowy.local", Description: "Admin email", Required: false},
			{Key: "APPFLOWY_GOTRUE_ADMIN_PASSWORD", Default: "", Description: "Admin password", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:8000/health"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "appflowy-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "appflowy", "POSTGRES_USER": "appflowy", "POSTGRES_PASSWORD": "appflowy_password"},
				Volumes:     []Volume{{Name: "appflowy-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U appflowy"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "appflowy-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "appflowy-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func seafile() *AppTemplate {
	return &AppTemplate{
		ID:          "seafile",
		Name:        "Seafile",
		Description: "High-performance self-hosted file sync and share platform",
		Category:    "Storage",
		Icon:        "🌊",
		Version:     "latest",
		Image:       "seafileltd/seafile-mc:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "seafile-data", MountPath: "/shared", Description: "Seafile libraries and configuration"},
		},
		EnvVars: []EnvVar{
			{Key: "DB_HOST", Default: "seafile-db", Description: "MySQL/MariaDB host", Required: false},
			{Key: "DB_ROOT_PASSWD", Default: "", Description: "MySQL root password", Required: true, Secret: true},
			{Key: "SEAFILE_ADMIN_EMAIL", Default: "admin@seafile.local", Description: "Admin email address", Required: false},
			{Key: "SEAFILE_ADMIN_PASSWORD", Default: "", Description: "Admin password", Required: true, Secret: true},
			{Key: "SEAFILE_SERVER_HOSTNAME", Default: "", Description: "Public hostname", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/api2/ping/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "seafile-db", Image: "mariadb:10.11", Optional: true, Role: "database",
				Environment: map[string]string{"MYSQL_ROOT_PASSWORD": "seafile_root_password", "MYSQL_LOG_CONSOLE": "true"},
				Volumes:     []Volume{{Name: "seafile-db-data", MountPath: "/var/lib/mysql", Description: "MariaDB data"}},
			},
			{Name: "seafile-memcached", Image: "memcached:1.6-alpine", Optional: true, Role: "cache"},
		},
	}
}

func filebrowser() *AppTemplate {
	return &AppTemplate{
		ID:          "filebrowser",
		Name:        "Filebrowser",
		Description: "Lightweight web-based file manager with sharing and user management",
		Category:    "Storage",
		Icon:        "📁",
		Version:     "latest",
		Image:       "filebrowser/filebrowser:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "filebrowser-data", MountPath: "/srv", Description: "Files served by Filebrowser"},
			{Name: "filebrowser-db", MountPath: "/database", Description: "Filebrowser database"},
			{Name: "filebrowser-config", MountPath: "/config", Description: "Filebrowser configuration"},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func navidrome() *AppTemplate {
	return &AppTemplate{
		ID:          "navidrome",
		Name:        "Navidrome",
		Description: "Self-hosted music server and streamer compatible with Subsonic/Airsonic",
		Category:    "Media",
		Icon:        "🎵",
		Version:     "latest",
		Image:       "deluan/navidrome:latest",
		ProxyPort:   4533,
		Ports:       []Port{{Internal: 4533, External: 4533, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "navidrome-data", MountPath: "/data", Description: "Navidrome database and configuration"},
			{Name: "navidrome-music", MountPath: "/music", Description: "Music library"},
		},
		EnvVars: []EnvVar{
			{Key: "ND_SCANSCHEDULE", Default: "@every 1h", Description: "Cron schedule for library scans", Required: false},
			{Key: "ND_LOGLEVEL", Default: "info", Description: "Log level", Required: false},
			{Key: "ND_BASEURL", Default: "", Description: "Base URL path", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:4533/ping"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func kavita() *AppTemplate {
	return &AppTemplate{
		ID:          "kavita",
		Name:        "Kavita",
		Description: "Self-hosted digital library for manga, comics, and books",
		Category:    "Media",
		Icon:        "📚",
		Version:     "latest",
		Image:       "jvmilazz0/kavita:latest",
		ProxyPort:   5000,
		Ports:       []Port{{Internal: 5000, External: 5000, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "kavita-config", MountPath: "/kavita/config", Description: "Kavita configuration and database"},
			{Name: "kavita-library", MountPath: "/library", Description: "Manga, comics, and book library"},
		},
		EnvVars: []EnvVar{
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:5000/api/health"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func plex() *AppTemplate {
	return &AppTemplate{
		ID:          "plex",
		Name:        "Plex",
		Description: "Personal media server for movies, TV, music, and photos",
		Category:    "Media",
		Icon:        "🎞️",
		Version:     "latest",
		Image:       "plexinc/pms-docker:latest",
		ProxyPort:   32400,
		Ports:       []Port{{Internal: 32400, External: 32400, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "plex-config", MountPath: "/config", Description: "Plex configuration and metadata database"},
			{Name: "plex-transcode", MountPath: "/transcode", Description: "Temporary transcoding files"},
			{Name: "plex-media", MountPath: "/data", Description: "Media library"},
		},
		EnvVars: []EnvVar{
			{Key: "PLEX_CLAIM", Default: "", Description: "Claim token from plex.tv/claim to link your server", Required: false, Secret: true},
			{Key: "ADVERTISE_IP", Default: "", Description: "Public IP/URL for Plex to advertise", Required: false},
			{Key: "TZ", Default: "UTC", Description: "Timezone", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:32400/identity"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func ollama() *AppTemplate {
	return &AppTemplate{
		ID:          "ollama",
		Name:        "Ollama",
		Description: "Run large language models locally with a simple API",
		Category:    "AI",
		Icon:        "🦙",
		Version:     "latest",
		Image:       "ollama/ollama:latest",
		ProxyPort:   11434,
		Ports:       []Port{{Internal: 11434, External: 11434, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "ollama-data", MountPath: "/root/.ollama", Description: "Ollama model storage"},
		},
		EnvVars: []EnvVar{
			{Key: "OLLAMA_HOST", Default: "0.0.0.0:11434", Description: "Host and port to bind to", Required: false},
			{Key: "OLLAMA_NUM_PARALLEL", Default: "1", Description: "Maximum number of parallel requests", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:11434/api/version"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
	}
}

func activepieces() *AppTemplate {
	return &AppTemplate{
		ID:          "activepieces",
		Name:        "Activepieces",
		Description: "Open-source no-code business automation tool",
		Category:    "Automation",
		Icon:        "⚙️",
		Version:     "latest",
		Image:       "activepieces/activepieces:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "activepieces-data", MountPath: "/usr/src/app/data", Description: "Activepieces application data"},
		},
		EnvVars: []EnvVar{
			{Key: "AP_JWT_SECRET", Default: "", Description: "JWT secret for authentication", Required: true, Secret: true},
			{Key: "AP_ENCRYPTION_KEY", Default: "", Description: "Encryption key for credentials (32 chars)", Required: true, Secret: true},
			{Key: "AP_POSTGRES_HOST", Default: "activepieces-db", Description: "PostgreSQL host", Required: false},
			{Key: "AP_POSTGRES_DATABASE", Default: "activepieces", Description: "PostgreSQL database name", Required: false},
			{Key: "AP_POSTGRES_USERNAME", Default: "activepieces", Description: "PostgreSQL username", Required: false},
			{Key: "AP_POSTGRES_PASSWORD", Default: "activepieces_password", Description: "PostgreSQL password", Required: false, Secret: true},
			{Key: "AP_REDIS_URL", Default: "redis://activepieces-redis:6379", Description: "Redis connection string", Required: false},
			{Key: "AP_FRONTEND_URL", Default: "", Description: "Public URL of your Activepieces instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/api/v1/flags"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "activepieces-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "activepieces", "POSTGRES_USER": "activepieces", "POSTGRES_PASSWORD": "activepieces_password"},
				Volumes:     []Volume{{Name: "activepieces-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U activepieces"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "activepieces-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "activepieces-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func vikunja() *AppTemplate {
	return &AppTemplate{
		ID:          "vikunja",
		Name:        "Vikunja",
		Description: "Open-source to-do and project management app",
		Category:    "Productivity",
		Icon:        "✅",
		Version:     "latest",
		Image:       "vikunja/vikunja:latest",
		ProxyPort:   3456,
		Ports:       []Port{{Internal: 3456, External: 3456, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "vikunja-files", MountPath: "/app/vikunja/files", Description: "Vikunja file attachments"},
		},
		EnvVars: []EnvVar{
			{Key: "VIKUNJA_SERVICE_JWTSECRET", Default: "", Description: "JWT secret for authentication", Required: true, Secret: true},
			{Key: "VIKUNJA_DATABASE_TYPE", Default: "postgres", Description: "Database type: sqlite, mysql, or postgres", Required: false},
			{Key: "VIKUNJA_DATABASE_HOST", Default: "vikunja-db", Description: "Database host", Required: false},
			{Key: "VIKUNJA_DATABASE_DATABASE", Default: "vikunja", Description: "Database name", Required: false},
			{Key: "VIKUNJA_DATABASE_USER", Default: "vikunja", Description: "Database username", Required: false},
			{Key: "VIKUNJA_DATABASE_PASSWORD", Default: "vikunja_password", Description: "Database password", Required: false, Secret: true},
			{Key: "VIKUNJA_SERVICE_FRONTENDURL", Default: "", Description: "Public URL of your Vikunja instance", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:3456/api/v1/info"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "vikunja-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "vikunja", "POSTGRES_USER": "vikunja", "POSTGRES_PASSWORD": "vikunja_password"},
				Volumes:     []Volume{{Name: "vikunja-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U vikunja"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func taiga() *AppTemplate {
	return &AppTemplate{
		ID:          "taiga",
		Name:        "Taiga",
		Description: "Open-source agile project management platform",
		Category:    "Productivity",
		Icon:        "🌿",
		Version:     "latest",
		Image:       "taigaio/taiga-front:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "taiga-static", MountPath: "/taiga-back/static", Description: "Taiga static files"},
			{Name: "taiga-media", MountPath: "/taiga-back/media", Description: "Taiga media uploads"},
		},
		EnvVars: []EnvVar{
			{Key: "TAIGA_SECRET_KEY", Default: "", Description: "Django secret key", Required: true, Secret: true},
			{Key: "TAIGA_SITES_DOMAIN", Default: "", Description: "Public domain (e.g. taiga.example.com)", Required: true},
			{Key: "POSTGRES_HOST", Default: "taiga-db", Description: "PostgreSQL host", Required: false},
			{Key: "POSTGRES_DB", Default: "taiga", Description: "PostgreSQL database name", Required: false},
			{Key: "POSTGRES_USER", Default: "taiga", Description: "PostgreSQL username", Required: false},
			{Key: "POSTGRES_PASSWORD", Default: "taiga_password", Description: "PostgreSQL password", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "taiga-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "taiga", "POSTGRES_USER": "taiga", "POSTGRES_PASSWORD": "taiga_password"},
				Volumes:     []Volume{{Name: "taiga-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U taiga"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
			{
				Name: "taiga-redis", Image: "redis:7-alpine", Optional: true, Role: "cache",
				Volumes:     []Volume{{Name: "taiga-redis-data", MountPath: "/data", Description: "Redis data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD", "redis-cli", "ping"}, Interval: "10s", Timeout: "5s", Retries: 3},
			},
		},
	}
}

func leantime() *AppTemplate {
	return &AppTemplate{
		ID:          "leantime",
		Name:        "Leantime",
		Description: "Open-source project management for non-project managers",
		Category:    "Productivity",
		Icon:        "📌",
		Version:     "latest",
		Image:       "leantime/leantime:latest",
		ProxyPort:   80,
		Ports:       []Port{{Internal: 80, External: 8080, Protocol: "tcp"}},
		Volumes: []Volume{
			{Name: "leantime-public", MountPath: "/var/www/html/public/userfiles", Description: "Leantime user uploaded files"},
			{Name: "leantime-private", MountPath: "/var/www/html/userfiles", Description: "Leantime private user files"},
		},
		EnvVars: []EnvVar{
			{Key: "LEAN_DB_HOST", Default: "leantime-db", Description: "MySQL/MariaDB host", Required: false},
			{Key: "LEAN_DB_USER", Default: "leantime", Description: "Database username", Required: false},
			{Key: "LEAN_DB_PASSWORD", Default: "leantime_password", Description: "Database password", Required: false, Secret: true},
			{Key: "LEAN_DB_DATABASE", Default: "leantime", Description: "Database name", Required: false},
			{Key: "LEAN_APP_URL", Default: "", Description: "Public URL of your Leantime instance", Required: false},
			{Key: "LEAN_SESSION_PASSWORD", Default: "", Description: "Session encryption password", Required: true, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "leantime-db", Image: "mariadb:10.11", Optional: true, Role: "database",
				Environment: map[string]string{
					"MYSQL_DATABASE": "leantime", "MYSQL_USER": "leantime",
					"MYSQL_PASSWORD": "leantime_password", "MYSQL_RANDOM_ROOT_PASSWORD": "yes",
				},
				Volumes: []Volume{{Name: "leantime-db-data", MountPath: "/var/lib/mysql", Description: "MariaDB data"}},
			},
		},
	}
}

func forgejo() *AppTemplate {
	return &AppTemplate{
		ID:          "forgejo",
		Name:        "Forgejo",
		Description: "Lightweight self-hosted Git service (Gitea fork)",
		Category:    "Development",
		Icon:        "🔱",
		Version:     "latest",
		Image:       "codeberg.org/forgejo/forgejo:latest",
		ProxyPort:   3000,
		Ports: []Port{
			{Internal: 3000, External: 3000, Protocol: "tcp"},
			{Internal: 22, External: 2222, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "forgejo-data", MountPath: "/data", Description: "Forgejo repositories, configuration, and data"},
		},
		EnvVars: []EnvVar{
			{Key: "FORGEJO__database__DB_TYPE", Default: "sqlite3", Description: "Database type: sqlite3, postgres, or mysql", Required: false},
			{Key: "FORGEJO__database__HOST", Default: "forgejo-db:5432", Description: "Database host", Required: false},
			{Key: "FORGEJO__database__NAME", Default: "forgejo", Description: "Database name", Required: false},
			{Key: "FORGEJO__database__USER", Default: "forgejo", Description: "Database username", Required: false},
			{Key: "FORGEJO__database__PASSWD", Default: "forgejo_password", Description: "Database password", Required: false, Secret: true},
			{Key: "FORGEJO__server__DOMAIN", Default: "", Description: "Domain name for Forgejo", Required: false},
			{Key: "FORGEJO__server__ROOT_URL", Default: "", Description: "Full public URL", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost:3000"}, Interval: "30s", Timeout: "10s", Retries: 5,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "forgejo-db", Image: "postgres:15-alpine", Optional: true, Role: "database",
				Environment: map[string]string{"POSTGRES_DB": "forgejo", "POSTGRES_USER": "forgejo", "POSTGRES_PASSWORD": "forgejo_password"},
				Volumes:     []Volume{{Name: "forgejo-db-data", MountPath: "/var/lib/postgresql/data", Description: "PostgreSQL data"}},
				HealthCheck: HealthCheck{Test: []string{"CMD-SHELL", "pg_isready -U forgejo"}, Interval: "10s", Timeout: "5s", Retries: 5},
			},
		},
	}
}

func woodpecker() *AppTemplate {
	return &AppTemplate{
		ID:          "woodpecker",
		Name:        "Woodpecker CI",
		Description: "Simple and powerful CI/CD engine with Docker-based pipelines",
		Category:    "DevOps",
		Icon:        "🪵",
		Version:     "latest",
		Image:       "woodpeckerci/woodpecker-server:latest",
		ProxyPort:   8000,
		Ports: []Port{
			{Internal: 8000, External: 8000, Protocol: "tcp"},
			{Internal: 9000, External: 9000, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "woodpecker-data", MountPath: "/var/lib/woodpecker", Description: "Woodpecker server data"},
		},
		EnvVars: []EnvVar{
			{Key: "WOODPECKER_HOST", Default: "", Description: "Public URL of your Woodpecker instance", Required: true},
			{Key: "WOODPECKER_AGENT_SECRET", Default: "", Description: "Shared secret between server and agents", Required: true, Secret: true},
			{Key: "WOODPECKER_GITEA", Default: "true", Description: "Enable Gitea integration", Required: false},
			{Key: "WOODPECKER_GITEA_URL", Default: "", Description: "Gitea instance URL", Required: false},
			{Key: "WOODPECKER_GITEA_CLIENT", Default: "", Description: "Gitea OAuth2 client ID", Required: false},
			{Key: "WOODPECKER_GITEA_SECRET", Default: "", Description: "Gitea OAuth2 client secret", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "wget", "--spider", "-q", "http://localhost:8000/healthz"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "woodpecker-agent", Image: "woodpeckerci/woodpecker-agent:latest", Optional: true, Role: "agent",
				Volumes: []Volume{{Name: "docker-sock", MountPath: "/var/run/docker.sock", Description: "Docker socket"}},
			},
		},
	}
}

func drone() *AppTemplate {
	return &AppTemplate{
		ID:          "drone",
		Name:        "Drone CI",
		Description: "Container-native CI/CD platform with YAML-based pipelines",
		Category:    "DevOps",
		Icon:        "🚁",
		Version:     "latest",
		Image:       "drone/drone:latest",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 443, External: 8443, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "drone-data", MountPath: "/data", Description: "Drone server data and SQLite database"},
		},
		EnvVars: []EnvVar{
			{Key: "DRONE_RPC_SECRET", Default: "", Description: "Shared secret between server and runners", Required: true, Secret: true},
			{Key: "DRONE_SERVER_HOST", Default: "", Description: "Public hostname of your Drone instance", Required: true},
			{Key: "DRONE_SERVER_PROTO", Default: "http", Description: "Protocol (http or https)", Required: false},
			{Key: "DRONE_GITEA_SERVER", Default: "", Description: "Gitea server URL", Required: false},
			{Key: "DRONE_GITEA_CLIENT_ID", Default: "", Description: "Gitea OAuth2 client ID", Required: false},
			{Key: "DRONE_GITEA_CLIENT_SECRET", Default: "", Description: "Gitea OAuth2 client secret", Required: false, Secret: true},
			{Key: "DRONE_GITHUB_CLIENT_ID", Default: "", Description: "GitHub OAuth2 client ID", Required: false},
			{Key: "DRONE_GITHUB_CLIENT_SECRET", Default: "", Description: "GitHub OAuth2 client secret", Required: false, Secret: true},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/healthz"}, Interval: "30s", Timeout: "10s", Retries: 3,
		},
		ExtraServices: []ServiceDef{
			{
				Name: "drone-runner", Image: "drone/drone-runner-docker:latest", Optional: true, Role: "runner",
				Volumes: []Volume{{Name: "docker-sock", MountPath: "/var/run/docker.sock", Description: "Docker socket"}},
			},
		},
	}
}

func mailu() *AppTemplate {
	return &AppTemplate{
		ID:          "mailu",
		Name:        "Mailu",
		Description: "Simple, full-featured mail server suite (SMTP, IMAP, webmail, admin)",
		Category:    "Email",
		Icon:        "✉️",
		Version:     "2024.06",
		Image:       "ghcr.io/mailu/nginx:2024.06",
		ProxyPort:   80,
		Ports: []Port{
			{Internal: 80, External: 8080, Protocol: "tcp"},
			{Internal: 25, External: 25, Protocol: "tcp"},
			{Internal: 587, External: 587, Protocol: "tcp"},
			{Internal: 993, External: 993, Protocol: "tcp"},
		},
		Volumes: []Volume{
			{Name: "mailu-data", MountPath: "/data", Description: "Mailu application data"},
			{Name: "mailu-mail", MountPath: "/mail", Description: "Mail storage"},
			{Name: "mailu-certs", MountPath: "/certs", Description: "TLS certificates"},
		},
		EnvVars: []EnvVar{
			{Key: "SECRET_KEY", Default: "", Description: "Random 16-character secret key", Required: true, Secret: true},
			{Key: "DOMAIN", Default: "", Description: "Primary mail domain (e.g. example.com)", Required: true},
			{Key: "HOSTNAMES", Default: "", Description: "Comma-separated list of mail hostnames", Required: true},
			{Key: "INITIAL_ADMIN_PW", Default: "", Description: "Initial admin password", Required: true, Secret: true},
			{Key: "TLS_FLAVOR", Default: "cert", Description: "TLS mode: cert, letsencrypt, mail, notls", Required: false},
		},
		HealthCheck: HealthCheck{
			Test: []string{"CMD", "curl", "-f", "http://localhost/"}, Interval: "60s", Timeout: "30s", Retries: 10,
		},
		ExtraServices: []ServiceDef{
			{Name: "mailu-redis", Image: "redis:7-alpine", Optional: false, Role: "cache"},
			{Name: "mailu-admin", Image: "ghcr.io/mailu/admin:2024.06", Optional: false, Role: "admin"},
			{Name: "mailu-imap", Image: "ghcr.io/mailu/dovecot:2024.06", Optional: false, Role: "imap"},
			{Name: "mailu-smtp", Image: "ghcr.io/mailu/postfix:2024.06", Optional: false, Role: "smtp"},
			{Name: "mailu-antispam", Image: "ghcr.io/mailu/rspamd:2024.06", Optional: true, Role: "antispam"},
			{Name: "mailu-webmail", Image: "ghcr.io/mailu/webmail:2024.06", Optional: true, Role: "webmail"},
		},
	}
}
