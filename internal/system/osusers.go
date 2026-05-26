package system

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// OSUser represents a Linux OS-level user account.
type OSUser struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	Comment  string   `json:"comment"`
	HomeDir  string   `json:"home_dir"`
	Shell    string   `json:"shell"`
	Groups   []string `json:"groups"`
	System   bool     `json:"system"` // true if UID < 1000
}

// CreateOSUserRequest holds parameters for creating an OS user.
type CreateOSUserRequest struct {
	Username string `json:"username"`
	Comment  string `json:"comment"`  // GECOS / full name
	HomeDir  string `json:"home_dir"` // empty = default (/home/<username>)
	Shell    string `json:"shell"`    // empty = /bin/bash
	Groups   []string `json:"groups"` // supplementary groups
	System   bool   `json:"system"`   // create as system user (no home, UID < 1000)
	Password string `json:"password"` // if empty, account is locked
}

// UpdateOSUserRequest holds parameters for modifying an OS user.
type UpdateOSUserRequest struct {
	Comment  string   `json:"comment"`
	Shell    string   `json:"shell"`
	Groups   []string `json:"groups"`  // replaces supplementary groups
	Password string   `json:"password"` // if empty, password is not changed
	Lock     bool     `json:"lock"`     // lock the account
	Unlock   bool     `json:"unlock"`   // unlock the account
}

// ListOSUsers reads /etc/passwd and returns all non-system users (UID >= 1000)
// plus the root account. Pass includeSystem=true to include all system accounts.
func ListOSUsers(includeSystem bool) ([]OSUser, error) {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, fmt.Errorf("read /etc/passwd: %w", err)
	}
	defer f.Close()

	var users []OSUser
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		u, err := parsePasswdLine(line)
		if err != nil {
			continue
		}
		if !includeSystem && u.System && u.Username != "root" {
			continue
		}
		users = append(users, u)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Enrich with group memberships
	groupMap, _ := buildGroupMap()
	for i := range users {
		users[i].Groups = groupMap[users[i].Username]
	}

	return users, nil
}

// GetOSUser returns a single OS user by username.
func GetOSUser(username string) (*OSUser, error) {
	users, err := ListOSUsers(true)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.Username == username {
			cp := u
			return &cp, nil
		}
	}
	return nil, nil
}

// CreateOSUser creates a new Linux user account using useradd.
func CreateOSUser(req CreateOSUserRequest) (*OSUser, error) {
	if req.Username == "" {
		return nil, fmt.Errorf("username is required")
	}

	args := []string{}

	if req.System {
		args = append(args, "--system")
	} else {
		args = append(args, "--create-home")
	}

	if req.Comment != "" {
		args = append(args, "--comment", req.Comment)
	}
	if req.HomeDir != "" {
		args = append(args, "--home-dir", req.HomeDir)
	}
	shell := req.Shell
	if shell == "" && !req.System {
		shell = "/bin/bash"
	}
	if shell != "" {
		args = append(args, "--shell", shell)
	}
	if len(req.Groups) > 0 {
		args = append(args, "--groups", strings.Join(req.Groups, ","))
	}

	args = append(args, req.Username)

	if out, err := runCmdOutput("useradd", args...); err != nil {
		return nil, fmt.Errorf("useradd failed: %w — %s", err, strings.TrimSpace(out))
	}

	// Set password if provided
	if req.Password != "" {
		if err := setPassword(req.Username, req.Password); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}
	}

	return GetOSUser(req.Username)
}

// UpdateOSUser modifies an existing Linux user account using usermod.
func UpdateOSUser(username string, req UpdateOSUserRequest) (*OSUser, error) {
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	args := []string{}

	if req.Comment != "" {
		args = append(args, "--comment", req.Comment)
	}
	if req.Shell != "" {
		args = append(args, "--shell", req.Shell)
	}
	if len(req.Groups) > 0 {
		// -G replaces supplementary groups; -a -G appends
		args = append(args, "--groups", strings.Join(req.Groups, ","))
	}
	if req.Lock {
		args = append(args, "--lock")
	} else if req.Unlock {
		args = append(args, "--unlock")
	}

	if len(args) > 0 {
		args = append(args, username)
		if out, err := runCmdOutput("usermod", args...); err != nil {
			return nil, fmt.Errorf("usermod failed: %w — %s", err, strings.TrimSpace(out))
		}
	}

	// Update password separately
	if req.Password != "" {
		if err := setPassword(username, req.Password); err != nil {
			return nil, fmt.Errorf("set password: %w", err)
		}
	}

	return GetOSUser(username)
}

// DeleteOSUser removes a Linux user account using userdel.
// If removeHome is true, the home directory is also deleted.
func DeleteOSUser(username string, removeHome bool) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	args := []string{}
	if removeHome {
		args = append(args, "--remove")
	}
	args = append(args, username)
	if out, err := runCmdOutput("userdel", args...); err != nil {
		return fmt.Errorf("userdel failed: %w — %s", err, strings.TrimSpace(out))
	}
	return nil
}

// ListOSGroups returns all groups from /etc/group.
func ListOSGroups() ([]string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return nil, fmt.Errorf("read /etc/group: %w", err)
	}
	defer f.Close()

	var groups []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) >= 1 {
			groups = append(groups, parts[0])
		}
	}
	return groups, scanner.Err()
}

// --- helpers ---

func parsePasswdLine(line string) (OSUser, error) {
	parts := strings.Split(line, ":")
	if len(parts) < 7 {
		return OSUser{}, fmt.Errorf("malformed passwd line")
	}
	uid, err := strconv.Atoi(parts[2])
	if err != nil {
		return OSUser{}, err
	}
	gid, err := strconv.Atoi(parts[3])
	if err != nil {
		return OSUser{}, err
	}
	return OSUser{
		Username: parts[0],
		UID:      uid,
		GID:      gid,
		Comment:  parts[4],
		HomeDir:  parts[5],
		Shell:    parts[6],
		System:   uid < 1000,
	}, nil
}

// buildGroupMap returns a map of username → list of supplementary group names.
func buildGroupMap() (map[string][]string, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string][]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		groupName := parts[0]
		members := strings.Split(parts[3], ",")
		for _, m := range members {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			result[m] = append(result[m], groupName)
		}
	}
	return result, scanner.Err()
}

// setPassword sets a user's password using chpasswd.
func setPassword(username, password string) error {
	cmd := exec.Command("chpasswd")
	cmd.Stdin = strings.NewReader(fmt.Sprintf("%s:%s\n", username, password))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chpasswd: %w — %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runCmdOutput runs a command and returns combined output + error.
func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
