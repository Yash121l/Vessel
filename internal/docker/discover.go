package docker

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
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
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--format", `{"ID":"{{.ID}}","Names":"{{.Names}}","Image":"{{.Image}}","Status":"{{.Status}}","State":"{{.State}}","Ports":"{{.Ports}}","Labels":"{{.Labels}}","CreatedAt":"{{.CreatedAt}}"}`,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var containers []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var entry dockerPsEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
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
	return containers, nil
}

// ContainerLogs streams logs from a container by name or ID.
func ContainerLogs(ctx context.Context, nameOrID string, lines chan<- string) error {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--follow", "--timestamps", "--tail", "100", nameOrID)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
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
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// StopContainer stops a container by name or ID.
func StopContainer(ctx context.Context, nameOrID string) error {
	return exec.CommandContext(ctx, "docker", "stop", nameOrID).Run()
}

// StartContainer starts a stopped container by name or ID.
func StartContainer(ctx context.Context, nameOrID string) error {
	return exec.CommandContext(ctx, "docker", "start", nameOrID).Run()
}

// RestartContainer restarts a container by name or ID.
func RestartContainer(ctx context.Context, nameOrID string) error {
	return exec.CommandContext(ctx, "docker", "restart", nameOrID).Run()
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
