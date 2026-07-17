package iam

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Zaki-goumri/vexo/internal/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	return NewStore(meta)
}

func TestCreateAndGetUser(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "alice" {
		t.Fatalf("username: got %q, want %q", u.Username, "alice")
	}
	if u.Status != StatusActive {
		t.Fatalf("status: got %q, want %q", u.Status, StatusActive)
	}

	got, err := s.GetUser("alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice" {
		t.Fatalf("get: username %q", got.Username)
	}
}

func TestCreateDuplicateUser(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateUser("alice"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateUser("alice")
	if err != ErrUserAlreadyExists {
		t.Fatalf("want ErrUserAlreadyExists, got %v", err)
	}
}

func TestGetMissingUser(t *testing.T) {
	s := newTestStore(t)

	_, err := s.GetUser("ghost")
	if err != ErrUserNotFound {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestDeleteUserDeletesAccessKeys(t *testing.T) {
	s := newTestStore(t)

	s.CreateUser("alice")
	_, _, _ = s.CreateAccessKey("alice")
	_, _, _ = s.CreateAccessKey("alice")

	keys, _ := s.ListAccessKeys("alice")
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %d", len(keys))
	}

	if err := s.DeleteUser("alice"); err != nil {
		t.Fatal(err)
	}

	keys, _ = s.ListAccessKeys("alice")
	if len(keys) != 0 {
		t.Fatalf("want 0 keys after delete, got %d", len(keys))
	}
}

func TestCreateAccessKey(t *testing.T) {
	s := newTestStore(t)
	s.CreateUser("alice")

	ak, secret, err := s.CreateAccessKey("alice")
	if err != nil {
		t.Fatal(err)
	}
	if ak.AccessKey == "" {
		t.Fatal("access key ID is empty")
	}
	if secret == "" {
		t.Fatal("plaintext secret is empty")
	}
	if ak.PlainSecret != secret {
		t.Fatalf("stored secret should match plaintext (needed for SigV4)")
	}
	if ak.Username != "alice" {
		t.Fatalf("username: got %q, want %q", ak.Username, "alice")
	}
}

func TestValidateSecret(t *testing.T) {
	s := newTestStore(t)
	s.CreateUser("alice")

	_, secret, _ := s.CreateAccessKey("alice")
	keys, _ := s.ListAccessKeys("alice")
	akID := keys[0].AccessKey

	ok, err := s.ValidateSecret(akID, secret)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("valid secret should return true")
	}

	ok, _ = s.ValidateSecret(akID, "wrong-secret")
	if ok {
		t.Fatal("wrong secret should return false")
	}
}

func TestListAccessKeys(t *testing.T) {
	s := newTestStore(t)
	s.CreateUser("alice")

	for i := 0; i < 3; i++ {
		s.CreateAccessKey("alice")
	}

	keys, err := s.ListAccessKeys("alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("want 3 keys, got %d", len(keys))
	}
}

func TestDeleteAccessKey(t *testing.T) {
	s := newTestStore(t)
	s.CreateUser("alice")

	_, _, _ = s.CreateAccessKey("alice")
	keys, _ := s.ListAccessKeys("alice")
	akID := keys[0].AccessKey

	if err := s.DeleteAccessKey(akID); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetAccessKey(akID)
	if err != ErrAccessKeyNotFound {
		t.Fatalf("want ErrAccessKeyNotFound, got %v", err)
	}
}

func TestGroupCRUD(t *testing.T) {
	s := newTestStore(t)

	g, err := s.CreateGroup("admins")
	if err != nil {
		t.Fatal(err)
	}
	if g.Name != "admins" {
		t.Fatalf("group name: got %q", g.Name)
	}

	got, err := s.GetGroup("admins")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "admins" {
		t.Fatalf("get group: %q", got.Name)
	}
}

func TestCreateDuplicateGroup(t *testing.T) {
	s := newTestStore(t)

	s.CreateGroup("admins")
	_, err := s.CreateGroup("admins")
	if err != ErrGroupAlreadyExists {
		t.Fatalf("want ErrGroupAlreadyExists, got %v", err)
	}
}

func TestAddRemoveUserFromGroup(t *testing.T) {
	s := newTestStore(t)
	s.CreateGroup("admins")
	s.CreateUser("alice")

	if err := s.AddUserToGroup("admins", "alice"); err != nil {
		t.Fatal(err)
	}

	g, _ := s.GetGroup("admins")
	if len(g.Members) != 1 || g.Members[0] != "alice" {
		t.Fatalf("members: %v", g.Members)
	}

	s.AddUserToGroup("admins", "alice")
	g, _ = s.GetGroup("admins")
	if len(g.Members) != 1 {
		t.Fatalf("should not add duplicate, got %d", len(g.Members))
	}

	s.CreateUser("bob")
	s.AddUserToGroup("admins", "bob")
	g, _ = s.GetGroup("admins")
	if len(g.Members) != 2 {
		t.Fatalf("want 2 members, got %d", len(g.Members))
	}

	if err := s.RemoveUserFromGroup("admins", "alice"); err != nil {
		t.Fatal(err)
	}
	g, _ = s.GetGroup("admins")
	if len(g.Members) != 1 || g.Members[0] != "bob" {
		t.Fatalf("after remove: %v", g.Members)
	}
}

func TestDeleteGroup(t *testing.T) {
	s := newTestStore(t)
	s.CreateGroup("admins")

	if err := s.DeleteGroup("admins"); err != nil {
		t.Fatal(err)
	}

	_, err := s.GetGroup("admins")
	if err != ErrGroupNotFound {
		t.Fatalf("want ErrGroupNotFound, got %v", err)
	}
}

func TestBootstrapRootCreatesUserAndKey(t *testing.T) {
	s := newTestStore(t)
	tmp := t.TempDir()

	if err := s.BootstrapRoot(tmp); err != nil {
		t.Fatal(err)
	}

	users, _ := s.ListUsers()
	if len(users) != 1 {
		t.Fatalf("want 1 user, got %d", len(users))
	}
	if users[0].Username != "zaki" {
		t.Fatalf("username: got %q, want %q", users[0].Username, "zaki")
	}

	keys, _ := s.ListAccessKeys("zaki")
	if len(keys) != 1 {
		t.Fatalf("want 1 access key, got %d", len(keys))
	}

	keyFile := filepath.Join(tmp, ".vexo.root.key")
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("root key file missing: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perms: got %o, want 0600", info.Mode().Perm())
	}
}

func TestBootstrapRootIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	tmp := t.TempDir()

	if err := s.BootstrapRoot(tmp); err != nil {
		t.Fatal(err)
	}
	if err := s.BootstrapRoot(tmp); err != nil {
		t.Fatal(err)
	}

	users, _ := s.ListUsers()
	if len(users) != 1 {
		t.Fatalf("want 1 user, got %d", len(users))
	}
}