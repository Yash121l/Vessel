# Contributing Templates

Vessel templates have one committed source of truth:

```text
internal/registry/templates/<app-id>.yaml
```

To add a supported app, add one YAML file there. New Vessel releases embed that
file automatically, and the GitHub Pages workflow publishes it to the remote
catalog for already-installed servers.

## Template Checklist

- Use a stable lowercase `id` that matches the filename.
- Include `name`, `description`, `category`, `image`, `ports`, and `proxy_port`.
- Put user-editable settings in `env_vars`.
- Mark secrets with `secret: true`.
- Put bundled databases, Redis, and similar sidecars in `extra_services`.
- Set `optional: true` for sidecars that users can replace with external hosted
  services.
- Set a clear `role`, such as `database`, `cache`, `search`, or `worker`.

## Catalog Flow

`scripts/build-template-catalog.mjs` reads `internal/registry/templates/` and
generates the GitHub Pages catalog:

```text
docs/templates/index.json
docs/templates/<app-id>.yaml
```

The generated files are ignored locally and rebuilt by the Pages workflow. The
JSON index includes template metadata, SHA-256 hashes, and the YAML payload so
Vessel can refresh the public catalog with one small request.

## Local Validation

```bash
node scripts/build-template-catalog.mjs
GOCACHE=/private/tmp/vessel-gocache go test ./...
```
