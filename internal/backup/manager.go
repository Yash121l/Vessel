package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Yash121l/Vessel/internal/config"
)

const manifestName = "_manifest.json"

type Manager struct {
	cfg       *config.Config
	nginxRoot string
}

type Manifest struct {
	CreatedAt  time.Time       `json:"created_at"`
	DataDir    string          `json:"data_dir"`
	ConfigFile string          `json:"config_file"`
	NginxRoot  string          `json:"nginx_root"`
	Entries    []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	ArchivePath string `json:"archive_path"`
	TargetPath  string `json:"target_path"`
	Type        string `json:"type"`
	Mode        int64  `json:"mode"`
	Size        int64  `json:"size"`
	Linkname    string `json:"linkname,omitempty"`
}

type ArchiveInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type entry struct {
	archivePath string
	targetPath  string
	info        fs.FileInfo
	linkname    string
}

func NewManager(cfg *config.Config, nginxRoot string) *Manager {
	return &Manager{cfg: cfg, nginxRoot: filepath.Clean(nginxRoot)}
}

func (m *Manager) DefaultArchivePath() string {
	return filepath.Join(m.cfg.BackupsDir, fmt.Sprintf("vessel-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405")))
}

func (m *Manager) ListArchives() ([]ArchiveInfo, error) {
	if err := os.MkdirAll(m.cfg.BackupsDir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(m.cfg.BackupsDir)
	if err != nil {
		return nil, err
	}
	out := make([]ArchiveInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tar.gz") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		out = append(out, ArchiveInfo{
			Name:    e.Name(),
			Path:    filepath.Join(m.cfg.BackupsDir, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

func (m *Manager) CreateArchive(dest string) (*Manifest, error) {
	if dest == "" {
		dest = m.DefaultArchivePath()
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return nil, err
	}

	entries, manifest, err := m.collectEntries(dest)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(dest)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, item := range entries {
		if err := writeEntry(tw, item); err != nil {
			return nil, err
		}
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	hdr := &tar.Header{
		Name:    manifestName,
		Mode:    0644,
		Size:    int64(len(manifestBytes)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(manifestBytes); err != nil {
		return nil, err
	}

	return manifest, nil
}

func (m *Manager) RestoreArchive(src string, force bool) (*Manifest, error) {
	manifest, headers, err := readArchiveHeaders(src)
	if err != nil {
		return nil, err
	}

	for _, hdr := range headers {
		target := archiveTargetPath(hdr.Name)
		if target == "" {
			continue
		}
		if err := m.validateRestorePath(target, hdr, force); err != nil {
			return nil, err
		}
	}

	f, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		target := archiveTargetPath(hdr.Name)
		if hdr.Name == manifestName || target == "" {
			continue
		}
		if err := m.validateRestorePath(target, hdr, force); err != nil {
			return nil, err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := m.prepareRestorePath(target, hdr.Typeflag, force); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return nil, err
			}
			if err := os.Chmod(target, os.FileMode(hdr.Mode)); err != nil {
				return nil, err
			}
		case tar.TypeSymlink:
			if err := m.prepareRestorePath(target, hdr.Typeflag, force); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := m.prepareRestorePath(target, hdr.Typeflag, force); err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return nil, err
			}
			if err := out.Close(); err != nil {
				return nil, err
			}
		}
	}
	return manifest, nil
}

func (m *Manager) validateRestorePath(target string, hdr *tar.Header, force bool) error {
	if !m.isAllowedRestorePath(target) {
		return fmt.Errorf("archive contains path outside managed roots: %s", target)
	}
	if err := m.ensureSafeRestoreParent(target); err != nil {
		return err
	}
	if hdr.Typeflag == tar.TypeSymlink {
		if err := m.validateRestoreLinkTarget(target, hdr.Linkname); err != nil {
			return err
		}
	}
	return m.validateRestoreTarget(target, hdr, force)
}

func (m *Manager) isAllowedRestorePath(target string) bool {
	target = filepath.Clean(target)
	roots := []string{
		filepath.Clean(m.cfg.DataDir),
		filepath.Clean(m.nginxRoot),
	}
	for _, root := range roots {
		if root != "." && root != "" && isWithinRoot(target, root) {
			return true
		}
	}
	cfgPath := filepath.Clean(m.cfg.ConfigFile)
	return cfgPath != "." && cfgPath != "" && target == cfgPath
}

func isWithinRoot(target, root string) bool {
	if target == root {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (m *Manager) ensureSafeRestoreParent(target string) error {
	parent := filepath.Clean(filepath.Dir(target))
	root := m.managedRestoreRoot(target)
	if root == "" {
		return fmt.Errorf("restore target is outside managed roots: %s", target)
	}
	if info, err := os.Lstat(root); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore root is a symlink: %s", root)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if parent == "." || parent == string(filepath.Separator) || parent == root {
		return nil
	}

	rel, err := filepath.Rel(root, parent)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	current := root

	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore target traverses symlinked directory: %s", current)
		}
	}
	return nil
}

func (m *Manager) managedRestoreRoot(target string) string {
	target = filepath.Clean(target)
	for _, root := range []string{filepath.Clean(m.cfg.DataDir), filepath.Clean(m.nginxRoot)} {
		if root != "." && root != "" && isWithinRoot(target, root) {
			return root
		}
	}
	cfgPath := filepath.Clean(m.cfg.ConfigFile)
	if cfgPath != "." && cfgPath != "" && target == cfgPath {
		return filepath.Dir(cfgPath)
	}
	return ""
}

func (m *Manager) validateRestoreLinkTarget(target, linkname string) error {
	if linkname == "" {
		return fmt.Errorf("restore symlink target is empty: %s", target)
	}
	resolved := linkname
	if filepath.IsAbs(linkname) {
		resolved = filepath.Clean(linkname)
	} else {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(target), linkname))
	}
	if !m.isAllowedRestorePath(resolved) {
		return fmt.Errorf("restore symlink target escapes managed roots: %s -> %s", target, linkname)
	}
	return nil
}

func (m *Manager) validateRestoreTarget(target string, hdr *tar.Header, force bool) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore target is a symlink and cannot be replaced by a directory restore: %s", target)
		}
		if !force {
			return fmt.Errorf("restore target has incompatible existing entry: %s", target)
		}
		return nil
	case tar.TypeReg, tar.TypeRegA:
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 && !force {
			return fmt.Errorf("restore target is a symlink: %s (rerun with --force to replace it)", target)
		}
		return nil
	case tar.TypeSymlink:
		if info.IsDir() && !force {
			return fmt.Errorf("restore target is a directory: %s (rerun with --force to replace it)", target)
		}
		return nil
	default:
		return nil
	}
}

func (m *Manager) prepareRestorePath(target string, typeflag byte, force bool) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	switch typeflag {
	case tar.TypeDir:
		if info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore target is a symlink and cannot be replaced by a directory restore: %s", target)
		}
		if !force {
			return fmt.Errorf("restore target has incompatible existing entry: %s", target)
		}
		return os.Remove(target)
	case tar.TypeReg, tar.TypeRegA:
		if info.Mode()&os.ModeSymlink != 0 {
			if !force {
				return fmt.Errorf("restore target is a symlink: %s (rerun with --force to replace it)", target)
			}
			return os.Remove(target)
		}
		if info.IsDir() {
			// Restoring a file into an existing directory is only valid when the
			// archive entry itself is that directory, so treat this as a conflict.
			return fmt.Errorf("cannot restore file onto existing directory: %s", target)
		}
		return nil
	case tar.TypeSymlink:
		if info.IsDir() {
			if !force {
				return fmt.Errorf("cannot replace directory with symlink: %s", target)
			}
			return os.Remove(target)
		}
		return os.Remove(target)
	default:
		return nil
	}
}

func (m *Manager) collectEntries(dest string) ([]entry, *Manifest, error) {
	targets := []string{}
	if m.cfg.ConfigFile != "" {
		targets = append(targets, m.cfg.ConfigFile)
	}
	targets = append(targets, m.cfg.DataDir)
	if m.nginxRoot != "" {
		targets = append(targets, m.nginxRoot)
	}

	var entries []entry
	manifest := &Manifest{
		CreatedAt:  time.Now().UTC(),
		DataDir:    m.cfg.DataDir,
		ConfigFile: m.cfg.ConfigFile,
		NginxRoot:  m.nginxRoot,
	}
	seen := map[string]bool{}
	skip := map[string]bool{
		filepath.Clean(dest): true,
	}
	if m.cfg.BackupsDir != "" {
		skip[filepath.Clean(m.cfg.BackupsDir)] = true
	}

	for _, target := range targets {
		target = filepath.Clean(target)
		if target == "" {
			continue
		}
		if _, err := os.Lstat(target); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, nil, err
		}
		err := filepath.Walk(target, func(path string, info fs.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			clean := filepath.Clean(path)
			if skip[clean] {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if seen[clean] {
				return nil
			}
			seen[clean] = true
			linkname := ""
			if info.Mode()&os.ModeSymlink != 0 {
				var err error
				linkname, err = os.Readlink(clean)
				if err != nil {
					return err
				}
			}
			archivePath := archivePathForTarget(clean)
			entries = append(entries, entry{
				archivePath: archivePath,
				targetPath:  clean,
				info:        info,
				linkname:    linkname,
			})
			manifest.Entries = append(manifest.Entries, ManifestEntry{
				ArchivePath: archivePath,
				TargetPath:  clean,
				Type:        entryType(info),
				Mode:        int64(info.Mode().Perm()),
				Size:        info.Size(),
				Linkname:    linkname,
			})
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].archivePath < entries[j].archivePath })
	sort.Slice(manifest.Entries, func(i, j int) bool { return manifest.Entries[i].ArchivePath < manifest.Entries[j].ArchivePath })
	return entries, manifest, nil
}

func writeEntry(tw *tar.Writer, item entry) error {
	info := item.info
	hdr, err := tar.FileInfoHeader(info, item.linkname)
	if err != nil {
		return err
	}
	hdr.Name = item.archivePath
	hdr.ModTime = info.ModTime()
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	f, err := os.Open(item.targetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, f)
	return err
}

func readArchiveHeaders(src string) (*Manifest, []*tar.Header, error) {
	f, err := os.Open(src)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	headers := []*tar.Header{}
	var manifest *Manifest
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		hdrCopy := *hdr
		headers = append(headers, &hdrCopy)
		if hdr.Name == manifestName {
			var m Manifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				return nil, nil, err
			}
			manifest = &m
		}
	}
	if manifest == nil {
		return nil, nil, fmt.Errorf("backup manifest not found in archive")
	}
	return manifest, headers, nil
}

func archivePathForTarget(target string) string {
	target = filepath.Clean(target)
	target = filepath.ToSlash(target)
	return "rootfs" + target
}

func archiveTargetPath(name string) string {
	name = filepath.ToSlash(name)
	if !strings.HasPrefix(name, "rootfs/") && name != "rootfs" {
		return ""
	}
	trimmed := strings.TrimPrefix(name, "rootfs")
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func entryType(info fs.FileInfo) string {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return "symlink"
	case info.IsDir():
		return "dir"
	default:
		return "file"
	}
}
