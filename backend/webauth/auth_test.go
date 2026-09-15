package webauth

import (
	"path/filepath"
	"testing"
)

func TestRegisterLoginAndIsolationUsers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_ALLOW_REGISTER", "")
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	t.Setenv("WEB_ADMIN_USER", "")
	t.Setenv("WEB_ADMIN_PASSWORD", "")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureProvisioned(); err != nil {
		t.Fatal(err)
	}
	if !AllowRegister() {
		t.Fatal("first user should allow register when setup secret is set")
	}
	if _, err := Register("alice", "secret1", "wrong"); err != ErrInvalidSetupToken {
		t.Fatalf("want invalid setup token, got %v", err)
	}
	u, err := Register("alice", "secret1", "setup-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !u.IsAdmin {
		t.Fatal("first user should be admin")
	}
	if AllowRegister() {
		t.Fatal("register should close after first user")
	}
	if _, err := Register("bob", "secret2", ""); err != ErrRegisterClosed {
		t.Fatalf("expected closed, got %v", err)
	}
	bob, err := CreateUser("bob", "secret2", false)
	if err != nil {
		t.Fatal(err)
	}
	if bob.IsAdmin {
		t.Fatal("bob should not be admin")
	}
	got, err := Authenticate("alice", "secret1")
	if err != nil || got.ID != u.ID {
		t.Fatalf("login: %v %+v", err, got)
	}
	if _, err := Authenticate("alice", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("want invalid creds, got %v", err)
	}
	s, err := CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	me, err := UserBySession(s.Token)
	if err != nil || me.Username != "alice" {
		t.Fatalf("session: %v %+v", err, me)
	}
	DeleteSession(s.Token)
	if _, err := UserBySession(s.Token); err != ErrUnauthorized {
		t.Fatalf("deleted session: %v", err)
	}
}

func TestEnsureProvisionedRequiresAdminOrSetupSecret(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_ADMIN_USER", "")
	t.Setenv("WEB_ADMIN_PASSWORD", "")
	t.Setenv("WEB_SETUP_SECRET", "")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	if AllowRegister() {
		t.Fatal("empty database must not allow public register")
	}
	if err := EnsureProvisioned(); err == nil {
		t.Fatal("expected provision error")
	}
	if _, err := Register("alice", "secret1", ""); err != ErrRegisterClosed {
		t.Fatalf("want closed, got %v", err)
	}

	t.Setenv("WEB_ADMIN_USER", "admin")
	t.Setenv("WEB_ADMIN_PASSWORD", "secret1")
	u, err := BootstrapAdminFromEnv()
	if err != nil || u == nil || !u.IsAdmin {
		t.Fatalf("bootstrap: %v %+v", err, u)
	}
	if err := EnsureProvisioned(); err != nil {
		t.Fatal(err)
	}
	if AllowRegister() {
		t.Fatal("bootstrapped instance should keep register closed")
	}
}
