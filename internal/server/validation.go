package server

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	namePattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	labelPattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$`)
	envKeyPattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	imagePattern    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)
	fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
)

func validateDeploymentName(name string) error {
	if !labelPattern.MatchString(name) || strings.Contains(name, "--") {
		return fmt.Errorf("deployment name must use lowercase letters, numbers, and single hyphens")
	}
	return nil
}

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) || strings.Contains(username, "..") {
		return fmt.Errorf("username must be 3-32 characters and use letters, numbers, dots, underscores, or hyphens")
	}
	return nil
}

func validateRole(role string) error {
	if _, ok := roleRank[role]; !ok {
		return fmt.Errorf("role must be one of owner, admin, operator, viewer")
	}
	return nil
}

func validateDomain(domain string) error {
	if domain == "" {
		return nil
	}
	if strings.ContainsAny(domain, "/:@ ") {
		return fmt.Errorf("domain must be a hostname, not a URL")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain is too long")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return fmt.Errorf("domain must include at least one dot")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return fmt.Errorf("domain contains an invalid label")
		}
		if !labelPattern.MatchString(label) {
			return fmt.Errorf("domain labels must use lowercase letters, numbers, and hyphens")
		}
	}
	return nil
}

func validateEnv(env map[string]string) error {
	for k := range env {
		if !envKeyPattern.MatchString(k) {
			return fmt.Errorf("invalid environment variable name: %s", k)
		}
	}
	return nil
}

func validateImageRef(image string) error {
	if image == "" {
		return fmt.Errorf("image is required")
	}
	if !imagePattern.MatchString(image) || strings.Contains(image, "..") {
		return fmt.Errorf("invalid Docker image reference")
	}
	return nil
}

func validatePort(port int, field string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", field)
	}
	return nil
}

func validateSiteFileName(name string) error {
	if !fileNamePattern.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("site filename must use letters, numbers, dots, underscores, or hyphens")
	}
	return nil
}

func validateUpstream(upstream string) error {
	if upstream == "" {
		return nil
	}
	if strings.Contains(upstream, "://") {
		u, err := url.Parse(upstream)
		if err != nil || u.Host == "" {
			return fmt.Errorf("invalid upstream URL")
		}
		return nil
	}
	host, port, err := net.SplitHostPort(upstream)
	if err != nil || host == "" || port == "" {
		return fmt.Errorf("upstream must be host:port")
	}
	return nil
}
