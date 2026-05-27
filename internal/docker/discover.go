package docker

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/Yash121l/Vessel/internal/logger"
)

// Container represents a running Docker container discovered on the host.
type Container struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Image           string            `json:"image"`
	Status          string            `json:"status"`
	State           string            `json:"state"` // running, exited, paused
	Ports           []string          `json:"ports"`
	Labels          map[string]string `json:"labels"`
	CreatedAt       string            `json:"created_at"`
	ManagedByVessel bool              `json:"managed_by_vessel"`
}

type dockerPsEntry struct {
	ID      string `json:"ID"`
	Names   string `json:"Names"`
	Image   string `json:"Image"`
	Status  string `json:"Status"`
	State   string `json:"State"`
	Ports   string `json:"Ports"`
	Labels  string `json:"Labels"`
	Created string `json:"CreatedAt"`
}

// ListContainers returns all containers currently known to Docker.
func ListContainers(ctx context.Context) ([]Container, error) {
	logger.Debugf("Querying Docker host containers (docker ps -a)...")
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--format", `{"ID":"{{.ID}}","Names":"{{.Names}}","Image":"{{.Image}}","Status":"{{.Status}}","State":"{{.State}}","Ports":"{{.Ports}}","Labels":"{{.Labels}}","CreatedAt":"{{.CreatedAt}}"}`,
	)
	out, err := cmd.Output()
	if err != nil {
		logger.Errorf("docker ps command failed: %v", err)
		return nil, err
	}

	var containers []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var entry dockerPsEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			logger.Errorf("failed to parse docker ps entry json: %v", err)
			continue
		}

		labels := parseLabels(entry.Labels)
		c := Container{
			ID:              entry.ID,
			Name:            strings.TrimPrefix(entry.Names, "/"),
			Image:           entry.Image,
			Status:          entry.Status,
			State:           entry.State,
			Ports:           parsePorts(entry.Ports),
			Labels:          labels,
			CreatedAt:       entry.Created,
			ManagedByVessel: labels["vessel.managed"] == "true",
		}
		containers = append(containers, c)
	}
	logger.Debugf("Discovered %d containers on Docker host", len(containers))
	return containers, nil
}

// ContainerLogs streams logs from a container — tries name first, then ID.
func ContainerLogs(ctx context.Context, nameOrID string, lines chan<- string) error {
	resolved := resolveContainer(ctx, nameOrID)
	logger.Infof("Streaming Docker logs for resolved container '%s'...", resolved)

	cmd := exec.CommandContext(ctx, "docker", "logs", "--follow", "--timestamps", "--tail", "100", resolved)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Errorf("failed to get stdout pipe for container logs: %v", err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Errorf("failed to get stderr pipe for container logs: %v", err)
		return err
	}
	if err := cmd.Start(); err != nil {
		logger.Errorf("failed to start streaming docker logs: %v", err)
		return err
	}

	fanIn := func(r interface{ Read([]byte) (int, error) }) {
		buf := make([]byte, 4096)
		var acc strings.Builder
		for {
			n, err := r.Read(buf)
			if n > 0 {
				acc.Write(buf[:n])
				for {
					s := acc.String()
					idx := strings.Index(s, "\n")
					if idx < 0 {
						break
					}
					line := s[:idx]
					acc.Reset()
					acc.WriteString(s[idx+1:])
					select {
					case lines <- line:
					case <-ctx.Done():
						return
					}
				}
			}
			if err != nil {
				return
			}
		}
	}

	go fanIn(stdout)
	go fanIn(stderr)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		logger.Infof("Stopping container log stream for '%s' (context cancelled)", resolved)
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		if err != nil {
			logger.Errorf("container log stream exited with error: %v", err)
		} else {
			logger.Infof("container log stream for '%s' closed cleanly", resolved)
		}
		return err
	}
}

// StopContainer stops a container by name or ID.
func StopContainer(ctx context.Context, nameOrID string) error {
	resolved := resolveContainer(ctx, nameOrID)
	logger.Infof("Executing docker stop on container '%s'...", resolved)
	err := exec.CommandContext(ctx, "docker", "stop", resolved).Run()
	if err != nil {
		logger.Errorf("failed to stop container %s: %v", resolved, err)
	}
	return err
}

// StartContainer starts a stopped container by name or ID.
func StartContainer(ctx context.Context, nameOrID string) error {
	resolved := resolveContainer(ctx, nameOrID)
	logger.Infof("Executing docker start on container '%s'...", resolved)
	err := exec.CommandContext(ctx, "docker", "start", resolved).Run()
	if err != nil {
		logger.Errorf("failed to start container %s: %v", resolved, err)
	}
	return err
}

// RestartContainer restarts a container by name or ID.
func RestartContainer(ctx context.Context, nameOrID string) error {
	resolved := resolveContainer(ctx, nameOrID)
	logger.Infof("Executing docker restart on container '%s'...", resolved)
	err := exec.CommandContext(ctx, "docker", "restart", resolved).Run()
	if err != nil {
		logger.Errorf("failed to restart container %s: %v", resolved, err)
	}
	return err
}

// resolveContainer tries to find the best identifier for a container.
// If nameOrID looks like a short/full container ID that no longer exists,
// it falls back to matching by name across running containers.
func resolveContainer(ctx context.Context, nameOrID string) string {
	// First try the given value directly — if docker can find it, use it
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Name}}", nameOrID).Output()
	if err == nil && len(out) > 0 {
		return nameOrID
	}
	// Fallback: search all containers by name
	containers, err := ListContainers(ctx)
	if err != nil {
		return nameOrID
	}
	for _, c := range containers {
		if c.Name == nameOrID || strings.HasPrefix(c.ID, nameOrID) {
			return c.Name
		}
	}
	return nameOrID
}

// --- helpers ---

func parseLabels(raw string) map[string]string {
	labels := make(map[string]string)
	if raw == "" {
		return labels
	}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return labels
}

func parsePorts(raw string) []string {
	if raw == "" {
		return nil
	}
	var ports []string
	for _, p := range strings.Split(raw, ", ") {
		p = strings.TrimSpace(p)
		if p != "" {
			ports = append(ports, p)
		}
	}
	return ports
}
