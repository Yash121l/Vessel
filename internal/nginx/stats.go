package nginx

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Stats holds parsed nginx access log statistics.
type Stats struct {
	TotalRequests  int            `json:"total_requests"`
	Requests2xx    int            `json:"requests_2xx"`
	Requests3xx    int            `json:"requests_3xx"`
	Requests4xx    int            `json:"requests_4xx"`
	Requests5xx    int            `json:"requests_5xx"`
	TopPaths       []PathStat     `json:"top_paths"`
	TopIPs         []IPStat       `json:"top_ips"`
	RecentRequests []RecentReq    `json:"recent_requests"`
	StatusCodes    map[string]int `json:"status_codes"`
	BytesSent      int64          `json:"bytes_sent"`
	ActiveSites    []ActiveSite   `json:"active_sites"`
}

// PathStat is a URL path with request count.
type PathStat struct {
	Path  string `json:"path"`
	Count int    `json:"count"`
}

// IPStat is a client IP with request count.
type IPStat struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// RecentReq is a recent access log entry.
type RecentReq struct {
	IP     string `json:"ip"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Size   string `json:"size"`
	Time   string `json:"time"`
}

// ActiveSite is an nginx site with traffic info.
type ActiveSite struct {
	Name       string `json:"name"`
	ServerName string `json:"server_name"`
	Port       string `json:"port"`
	Upstream   string `json:"upstream"`
	Enabled    bool   `json:"enabled"`
	Requests   int    `json:"requests"`
}

// nginxLogRe matches the default nginx combined log format:
// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
var nginxLogRe = regexp.MustCompile(`^(\S+) - \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+)`)

// GetStats parses the nginx access log and returns statistics.
func (m *Manager) GetStats(lines int) Stats {
	s := Stats{
		StatusCodes: make(map[string]int),
	}

	logPath := "/var/log/nginx/access.log"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return s
	}

	// Use tail to get last N lines efficiently
	out, err := exec.Command("tail", "-n", strconv.Itoa(lines), logPath).Output()
	if err != nil {
		return s
	}

	pathCounts := make(map[string]int)
	ipCounts := make(map[string]int)
	var recent []RecentReq

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		m := nginxLogRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ip, timeStr, method, path, status, size := m[1], m[2], m[3], m[4], m[5], m[6]

		s.TotalRequests++
		s.StatusCodes[status]++

		code, _ := strconv.Atoi(status)
		bytes, _ := strconv.ParseInt(size, 10, 64)
		s.BytesSent += bytes

		switch {
		case code >= 200 && code < 300:
			s.Requests2xx++
		case code >= 300 && code < 400:
			s.Requests3xx++
		case code >= 400 && code < 500:
			s.Requests4xx++
		case code >= 500:
			s.Requests5xx++
		}

		pathCounts[path]++
		ipCounts[ip]++

		if len(recent) < 20 {
			recent = append([]RecentReq{{
				IP: ip, Method: method, Path: truncate(path, 60),
				Status: status, Size: size, Time: timeStr,
			}}, recent...)
		}
	}

	s.RecentRequests = recent
	s.TopPaths = topN(pathCounts, 10)
	s.TopIPs = topNIP(ipCounts, 5)
	s.ActiveSites = m.getActiveSites()
	return s
}

func (m *Manager) getActiveSites() []ActiveSite {
	sites, err := m.ListSites()
	if err != nil {
		return nil
	}

	serverNameRe := regexp.MustCompile(`server_name\s+([^;]+);`)
	listenRe := regexp.MustCompile(`listen\s+(\d+)`)
	upstreamRe := regexp.MustCompile(`proxy_pass\s+http://([^;/\s]+)`)

	var result []ActiveSite
	for _, site := range sites {
		data, err := os.ReadFile(site.Path)
		if err != nil {
			continue
		}
		content := string(data)

		as := ActiveSite{
			Name:    site.Name,
			Enabled: site.Enabled,
		}

		if m := serverNameRe.FindStringSubmatch(content); m != nil {
			as.ServerName = strings.TrimSpace(m[1])
		}
		if m := listenRe.FindStringSubmatch(content); m != nil {
			as.Port = m[1]
		}
		if m := upstreamRe.FindStringSubmatch(content); m != nil {
			as.Upstream = m[1]
		}
		result = append(result, as)
	}
	return result
}

func topN(m map[string]int, n int) []PathStat {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	// Simple insertion sort for small N
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].v > sorted[j-1].v; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	var result []PathStat
	for i, kv := range sorted {
		if i >= n {
			break
		}
		result = append(result, PathStat{Path: kv.k, Count: kv.v})
	}
	return result
}

func topNIP(m map[string]int, n int) []IPStat {
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].v > sorted[j-1].v; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	var result []IPStat
	for i, kv := range sorted {
		if i >= n {
			break
		}
		result = append(result, IPStat{IP: kv.k, Count: kv.v})
	}
	return result
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
