package backup

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yash121l/Vessel/internal/config"
)

func TestCreateAndRestoreArchive(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.DeploymentsDir = filepath.Join(cfg.DataDir, "deployments")
	cfg.TemplatesDir = filepath.Join(cfg.DataDir, "templates")
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	cfg.DBPath = filepath.Join(cfg.DataDir, "vessel.db")

	nginxRoot := filepath.Join(root, "etc", "nginx")
	if err := os.MkdirAll(cfg.DeploymentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.TemplatesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nginxRoot, "sites-available"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nginxRoot, "sites-enabled"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ConfigFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.ConfigFile, []byte("port: 4800\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DBPath, []byte("sqlite"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DeploymentsDir, "demo.txt"), []byte("deployment"), 0644); err != nil {
		t.Fatal(err)
	}
	sitePath := filepath.Join(nginxRoot, "sites-available", "demo.conf")
	if err := os.WriteFile(sitePath, []byte("server {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sitePath, filepath.Join(nginxRoot, "sites-enabled", "demo.conf")); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfg, nginxRoot)
	archivePath := filepath.Join(root, "backup.tar.gz")
	manifest, err := mgr.CreateArchive(archivePath)
	if err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}
	if len(manifest.Entries) == 0 {
		t.Fatal("expected manifest entries")
	}

	if err := os.RemoveAll(filepath.Join(root, "etc")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "var")); err != nil {
		t.Fatal(err)
	}

	restored, err := mgr.RestoreArchive(archivePath, false)
	if err != nil {
		t.Fatalf("RestoreArchive: %v", err)
	}
	if len(restored.Entries) != len(manifest.Entries) {
		t.Fatalf("restored manifest entry count = %d, want %d", len(restored.Entries), len(manifest.Entries))
	}

	if data, err := os.ReadFile(cfg.DBPath); err != nil || string(data) != "sqlite" {
		t.Fatalf("restored db mismatch, data=%q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(sitePath); err != nil || string(data) != "server {}" {
		t.Fatalf("restored site mismatch, data=%q err=%v", string(data), err)
	}
	linkTarget, err := os.Readlink(filepath.Join(nginxRoot, "sites-enabled", "demo.conf"))
	if err != nil {
		t.Fatalf("restored symlink missing: %v", err)
	}
	if linkTarget != sitePath {
		t.Fatalf("restored symlink target = %q, want %q", linkTarget, sitePath)
	}
}

func TestRestoreArchiveRejectsPathsOutsideManagedRoots(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	nginxRoot := filepath.Join(root, "etc", "nginx")
	mgr := NewManager(cfg, nginxRoot)

	archivePath := filepath.Join(root, "malicious.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	writeFile := func(name, body string) {
		t.Helper()
		hdr := &tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	writeFile("rootfs/etc/passwd", "oops")
	writeFile(manifestName, `{"created_at":"2026-01-01T00:00:00Z","data_dir":"/tmp","config_file":"/tmp/config.yaml","nginx_root":"/tmp/nginx","entries":[]}`)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, false); err == nil {
		t.Fatal("RestoreArchive() succeeded for path outside managed roots")
	}
}

func TestRestoreArchiveOverwritesManagedFilesWithoutDeletingRoots(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.DeploymentsDir = filepath.Join(cfg.DataDir, "deployments")
	cfg.TemplatesDir = filepath.Join(cfg.DataDir, "templates")
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	cfg.DBPath = filepath.Join(cfg.DataDir, "vessel.db")
	nginxRoot := filepath.Join(root, "etc", "nginx")

	for _, dir := range []string{
		cfg.DeploymentsDir,
		cfg.TemplatesDir,
		filepath.Join(nginxRoot, "conf.d"),
		filepath.Dir(cfg.ConfigFile),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(cfg.ConfigFile, []byte("port: 4800\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DBPath, []byte("good-db"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nginxRoot, "conf.d", "demo.conf"), []byte("server {}"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfg, nginxRoot)
	archivePath := filepath.Join(root, "restoreable.tar.gz")
	if _, err := mgr.CreateArchive(archivePath); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(cfg.ConfigFile, []byte("port: 9999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.DBPath, []byte("bad-db"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nginxRoot, "conf.d", "demo.conf"), []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nginxRoot, "conf.d", "keep.conf"), []byte("keep-me"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, false); err != nil {
		t.Fatalf("RestoreArchive() error = %v", err)
	}

	if data, err := os.ReadFile(cfg.DBPath); err != nil || string(data) != "good-db" {
		t.Fatalf("db after restore = %q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(cfg.ConfigFile); err != nil || string(data) != "port: 4800\n" {
		t.Fatalf("config after restore = %q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(filepath.Join(nginxRoot, "conf.d", "demo.conf")); err != nil || string(data) != "server {}" {
		t.Fatalf("nginx site after restore = %q err=%v", string(data), err)
	}
	if data, err := os.ReadFile(filepath.Join(nginxRoot, "conf.d", "keep.conf")); err != nil || string(data) != "keep-me" {
		t.Fatalf("unmanaged nginx file should remain untouched, data=%q err=%v", string(data), err)
	}
}

func TestRestoreArchiveRejectsSymlinkTargetsOutsideManagedRoots(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	nginxRoot := filepath.Join(root, "etc", "nginx")
	mgr := NewManager(cfg, nginxRoot)

	archivePath := filepath.Join(root, "symlink-escape.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	linkPath := archivePathForTarget(filepath.Join(cfg.DataDir, "pivot"))
	hdr := &tar.Header{
		Name:     linkPath,
		Typeflag: tar.TypeSymlink,
		Mode:     0777,
		Linkname: "/etc",
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	manifestBody := `{"created_at":"2026-01-01T00:00:00Z","data_dir":"` + cfg.DataDir + `","config_file":"` + cfg.ConfigFile + `","nginx_root":"` + nginxRoot + `","entries":[]}`
	manifestHdr := &tar.Header{Name: manifestName, Mode: 0644, Size: int64(len(manifestBody))}
	if err := tw.WriteHeader(manifestHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifestBody)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, false); err == nil {
		t.Fatal("RestoreArchive() succeeded for symlink target outside managed roots")
	}
}

func TestRestoreArchiveRejectsSymlinkedParentDirectories(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	nginxRoot := filepath.Join(root, "etc", "nginx")
	mgr := NewManager(cfg, nginxRoot)

	outsideRoot := filepath.Join(root, "outside")
	if err := os.MkdirAll(filepath.Join(outsideRoot, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(cfg.DataDir, "pivot")); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(root, "parent-symlink.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	body := "owned"
	fileHdr := &tar.Header{
		Name: archivePathForTarget(filepath.Join(cfg.DataDir, "pivot", "nested", "owned.txt")),
		Mode: 0644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(fileHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	manifestBody := `{"created_at":"2026-01-01T00:00:00Z","data_dir":"` + cfg.DataDir + `","config_file":"` + cfg.ConfigFile + `","nginx_root":"` + nginxRoot + `","entries":[]}`
	manifestHdr := &tar.Header{Name: manifestName, Mode: 0644, Size: int64(len(manifestBody))}
	if err := tw.WriteHeader(manifestHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifestBody)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, false); err == nil {
		t.Fatal("RestoreArchive() succeeded with a symlinked parent directory")
	}
}

func TestRestoreArchiveRejectsSymlinkedManagedRoot(t *testing.T) {
	root := t.TempDir()
	outsideRoot := filepath.Join(root, "outside")
	if err := os.MkdirAll(outsideRoot, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		DataDir:    filepath.Join(root, "managed-data"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.BackupsDir = filepath.Join(root, "backups")
	nginxRoot := filepath.Join(root, "etc", "nginx")
	if err := os.Symlink(outsideRoot, cfg.DataDir); err != nil {
		t.Fatal(err)
	}
	mgr := NewManager(cfg, nginxRoot)

	archivePath := filepath.Join(root, "root-symlink.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	body := "owned"
	fileHdr := &tar.Header{
		Name: archivePathForTarget(filepath.Join(cfg.DataDir, "owned.txt")),
		Mode: 0644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(fileHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	manifestBody := `{"created_at":"2026-01-01T00:00:00Z","data_dir":"` + cfg.DataDir + `","config_file":"` + cfg.ConfigFile + `","nginx_root":"` + nginxRoot + `","entries":[]}`
	manifestHdr := &tar.Header{Name: manifestName, Mode: 0644, Size: int64(len(manifestBody))}
	if err := tw.WriteHeader(manifestHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifestBody)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, false); err == nil {
		t.Fatal("RestoreArchive() succeeded with a symlinked managed root")
	}
}

func TestRestoreArchiveRejectsSymlinkedDirectoryTargetEvenWithForce(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{
		DataDir:    filepath.Join(root, "var", "lib", "vessel"),
		ConfigFile: filepath.Join(root, "etc", "vessel", "config.yaml"),
	}
	cfg.BackupsDir = filepath.Join(cfg.DataDir, "backups")
	nginxRoot := filepath.Join(root, "etc", "nginx")

	outsideRoot := filepath.Join(root, "outside")
	if err := os.MkdirAll(outsideRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideRoot, filepath.Join(cfg.DataDir, "dir")); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(cfg, nginxRoot)
	archivePath := filepath.Join(root, "dir-force.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	dirPath := archivePathForTarget(filepath.Join(cfg.DataDir, "dir"))
	dirHdr := &tar.Header{Name: dirPath, Typeflag: tar.TypeDir, Mode: 0755}
	if err := tw.WriteHeader(dirHdr); err != nil {
		t.Fatal(err)
	}
	body := "owned"
	fileHdr := &tar.Header{
		Name: archivePathForTarget(filepath.Join(cfg.DataDir, "dir", "owned.txt")),
		Mode: 0644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(fileHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	manifestBody := `{"created_at":"2026-01-01T00:00:00Z","data_dir":"` + cfg.DataDir + `","config_file":"` + cfg.ConfigFile + `","nginx_root":"` + nginxRoot + `","entries":[]}`
	manifestHdr := &tar.Header{Name: manifestName, Mode: 0644, Size: int64(len(manifestBody))}
	if err := tw.WriteHeader(manifestHdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(manifestBody)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := mgr.RestoreArchive(archivePath, true); err == nil {
		t.Fatal("RestoreArchive() succeeded for a symlinked directory target")
	}
	linkInfo, err := os.Lstat(filepath.Join(cfg.DataDir, "dir"))
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("directory target should remain a symlink, mode=%v", linkInfo.Mode())
	}
	if _, err := os.Stat(filepath.Join(outsideRoot, "owned.txt")); !os.IsNotExist(err) {
		t.Fatalf("outside root should remain untouched, stat err=%v", err)
	}
}
