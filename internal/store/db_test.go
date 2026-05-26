package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateAndSettings(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "vessel.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := db.SetSetting("admin_password_hash", "hash"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}
	got, err := db.GetSetting("admin_password_hash")
	if err != nil {
		t.Fatalf("GetSetting() error = %v", err)
	}
	if got != "hash" {
		t.Fatalf("GetSetting() = %q, want hash", got)
	}
}

func TestDeploymentEnvCascadeDelete(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "vessel.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	d := &Deployment{
		ID:         "dep-1",
		Name:       "demo",
		AppID:      "demo",
		Status:     "running",
		ComposeDir: "/tmp/demo",
		Env:        map[string]string{"APP_SECRET": "secret"},
	}
	if err := db.CreateDeployment(d); err != nil {
		t.Fatalf("CreateDeployment() error = %v", err)
	}
	if err := db.DeleteDeployment(d.ID); err != nil {
		t.Fatalf("DeleteDeployment() error = %v", err)
	}
	env, err := db.getEnv(d.ID)
	if err != nil {
		t.Fatalf("getEnv() error = %v", err)
	}
	if len(env) != 0 {
		t.Fatalf("env rows after delete = %#v, want none", env)
	}
}

func TestUsersAndSessions(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "vessel.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	user := &UserWithPassword{
		User: User{
			ID:       "user-1",
			Username: "admin",
			Role:     "owner",
		},
		PasswordHash: "hash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	count, err := db.CountUsers()
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountUsers() = %d, want 1", count)
	}

	expires := time.Now().Add(time.Hour)
	if err := db.CreateSession("token-hash", user.ID, expires); err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	got, err := db.GetSessionUser("token-hash")
	if err != nil {
		t.Fatalf("GetSessionUser() error = %v", err)
	}
	if got == nil || got.Username != "admin" || got.Role != "owner" {
		t.Fatalf("GetSessionUser() = %#v, want owner admin", got)
	}
	if err := db.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	got, err = db.GetSessionUser("token-hash")
	if err != nil {
		t.Fatalf("GetSessionUser() after delete error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSessionUser() after user delete = %#v, want nil", got)
	}
}
