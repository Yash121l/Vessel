package server

import (
	"strings"

	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
)

func deploymentResponse(reg *registry.Registry, d *store.Deployment) *store.Deployment {
	if d == nil {
		return nil
	}
	cp := *d
	if len(d.Env) > 0 {
		cp.Env = redactEnv(reg, d.AppID, d.Env)
	}
	return &cp
}

func deploymentListResponse(reg *registry.Registry, in []*store.Deployment) []*store.Deployment {
	out := make([]*store.Deployment, 0, len(in))
	for _, d := range in {
		out = append(out, deploymentResponse(reg, d))
	}
	return out
}

func redactEnv(reg *registry.Registry, appID string, env map[string]string) map[string]string {
	secretKeys := map[string]bool{}
	if tmpl, ok := reg.Get(appID); ok {
		for _, ev := range tmpl.EnvVars {
			if ev.Secret {
				secretKeys[strings.ToUpper(ev.Key)] = true
			}
		}
	}

	out := make(map[string]string, len(env))
	for k, v := range env {
		if isSecretKey(k, secretKeys) {
			out[k] = "********"
			continue
		}
		out[k] = v
	}
	return out
}

func redactComposeYAML(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.Trim(parts[0], `"'`)
		if isSecretKey(key, nil) {
			prefix := line[:strings.Index(line, strings.TrimLeft(line, " \t"))]
			lines[i] = prefix + key + ": ********"
		}
	}
	return strings.Join(lines, "\n")
}

func isSecretKey(key string, explicit map[string]bool) bool {
	upper := strings.ToUpper(key)
	if explicit != nil && explicit[upper] {
		return true
	}
	return strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "API_KEY") ||
		strings.Contains(upper, "ENCRYPTION_KEY")
}
