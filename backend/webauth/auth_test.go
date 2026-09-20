package webauth

import (
	"path/filepath"
	"strings"
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

func TestBootstrapAdminUpdatesExistingPassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	t.Setenv("WEB_ADMIN_USER", "")
	t.Setenv("WEB_ADMIN_PASSWORD", "")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	u, err := Register("alice", "oldpass1", "setup-secret")
	if err != nil {
		t.Fatal(err)
	}
	if u.IsAdmin {
		// first user is admin; demote to simulate a colliding non-admin name
		if err := authDB.Model(u).Update("is_admin", false).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("WEB_ADMIN_USER", "alice")
	t.Setenv("WEB_ADMIN_PASSWORD", "newpass1")
	got, err := BootstrapAdminFromEnv()
	if err != nil || got == nil || !got.IsAdmin {
		t.Fatalf("bootstrap: %v %+v", err, got)
	}
	if _, err := Authenticate("alice", "oldpass1"); err != ErrInvalidCredentials {
		t.Fatalf("old password should not work, got %v", err)
	}
	if _, err := Authenticate("alice", "newpass1"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestBootstrapAdminReenablesExistingDisabledUser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	t.Setenv("WEB_ADMIN_USER", "")
	t.Setenv("WEB_ADMIN_PASSWORD", "")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	u, err := Register("alice", "oldpass1", "setup-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := authDB.Model(u).Updates(map[string]any{
		"is_admin": false,
		"disabled": true,
	}).Error; err != nil {
		t.Fatal(err)
	}

	t.Setenv("WEB_ADMIN_USER", "alice")
	t.Setenv("WEB_ADMIN_PASSWORD", "newpass1")
	got, err := BootstrapAdminFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || !got.IsAdmin || got.Disabled {
		t.Fatalf("bootstrap should return enabled admin: %+v", got)
	}
	if _, err := Authenticate("alice", "newpass1"); err != nil {
		t.Fatalf("bootstrapped admin should authenticate: %v", err)
	}
}

func TestBootstrapAdminCreatesNewUser(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_SETUP_SECRET", "")
	t.Setenv("WEB_ADMIN_USER", "admin")
	t.Setenv("WEB_ADMIN_PASSWORD", "secret1")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}

	u, err := BootstrapAdminFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if u == nil || !u.IsAdmin || u.Disabled {
		t.Fatalf("bootstrap should create enabled admin: %+v", u)
	}
	if _, err := Authenticate("admin", "secret1"); err != nil {
		t.Fatalf("new bootstrap admin should authenticate: %v", err)
	}
	if UserCount() != 1 {
		t.Fatalf("want one user, got %d", UserCount())
	}
}

func TestBootstrapAdminRejectsInvalidPasswordWithoutMutatingExistingUser(t *testing.T) {
	for _, tc := range []struct {
		name     string
		password string
	}{
		{name: "short", password: strings.Repeat("x", MinPasswordLen-1)},
		{name: "overlong", password: strings.Repeat("x", MaxPasswordLen+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("WEB_SETUP_SECRET", "setup-secret")
			t.Setenv("WEB_ADMIN_USER", "")
			t.Setenv("WEB_ADMIN_PASSWORD", "")
			if err := Init(filepath.Join(dir, "auth.db")); err != nil {
				t.Fatal(err)
			}
			u, err := Register("alice", "oldpass1", "setup-secret")
			if err != nil {
				t.Fatal(err)
			}
			if err := authDB.Model(u).Updates(map[string]any{
				"is_admin": false,
				"disabled": true,
			}).Error; err != nil {
				t.Fatal(err)
			}
			before, err := GetUser(u.ID)
			if err != nil {
				t.Fatal(err)
			}

			t.Setenv("WEB_ADMIN_USER", "alice")
			t.Setenv("WEB_ADMIN_PASSWORD", tc.password)
			got, err := BootstrapAdminFromEnv()
			if err != ErrInvalidPassword || got != nil {
				t.Fatalf("want invalid password and nil user, got %v %+v", err, got)
			}

			after, err := GetUser(u.ID)
			if err != nil {
				t.Fatal(err)
			}
			if after.PasswordHash != before.PasswordHash || after.IsAdmin != before.IsAdmin || after.Disabled != before.Disabled {
				t.Fatalf("invalid bootstrap mutated user: before=%+v after=%+v", before, after)
			}
			if _, err := Authenticate("alice", "oldpass1"); err != ErrUserDisabled {
				t.Fatalf("old password and disabled state should remain, got %v", err)
			}
		})
	}
}

func TestRequireActiveUserRejectsDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WEB_SETUP_SECRET", "setup-secret")
	t.Setenv("WEB_ADMIN_USER", "")
	t.Setenv("WEB_ADMIN_PASSWORD", "")
	if err := Init(filepath.Join(dir, "auth.db")); err != nil {
		t.Fatal(err)
	}
	u, err := Register("alice", "secret1", "setup-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := RequireActiveUser(u.ID); err != nil {
		t.Fatal(err)
	}
	if err := SetDisabled(u.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := RequireActiveUser(u.ID); err != ErrUserDisabled {
		t.Fatalf("want disabled, got %v", err)
	}
	if err := SetDisabled(u.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := RequireActiveUser(u.ID); err != nil {
		t.Fatal(err)
	}
}
