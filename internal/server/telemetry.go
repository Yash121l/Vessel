package server

import (
	"time"

	"github.com/Yash121l/Vessel/internal/operations"
	"github.com/Yash121l/Vessel/internal/registry"
	"github.com/Yash121l/Vessel/internal/store"
	"github.com/Yash121l/Vessel/internal/telemetry"
)

// publicTemplateIDs returns the ids of the bundled and public catalog templates. Only these
// ids are ever reported; local templates in the data dir may be private.
func publicTemplateIDs(reg *registry.Registry) map[string]bool {
	ids := make(map[string]bool)
	for _, t := range reg.List() {
		ids[t.ID] = true
	}
	return ids
}

// reportOperations sends one operation_completed event per finished operation: its kind,
// whether it succeeded and how long it took. For template deploys it adds the public
// template id. Never deployment names, domains, images or error text.
func reportOperations(ops *operations.Manager, tel *telemetry.Client, publicIDs map[string]bool) {
	ops.OnFinish = func(spec operations.Spec, duration time.Duration, err error) {
		props := map[string]any{
			"kind":          spec.Kind,
			"resource_type": spec.ResourceType,
			"success":       err == nil,
			"duration_ms":   duration.Milliseconds(),
		}
		if appID, ok := spec.Metadata["app_id"].(string); ok && publicIDs[appID] {
			props["app_id"] = appID
		}
		tel.Capture("operation_completed", props)
	}
}

// reportStart sends server_started with instance-level counts.
func reportStart(tel *telemetry.Client, db *store.DB, reg *registry.Registry) {
	props := map[string]any{"template_count": len(reg.List())}
	if deployments, err := db.ListDeployments(); err == nil {
		props["deployment_count"] = len(deployments)
	}
	if users, err := db.CountUsers(); err == nil {
		props["user_count"] = users
	}
	tel.Capture("server_started", props)
}
