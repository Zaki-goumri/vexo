package policy

import (
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

func TestAllowMatches(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"arn:aws:s3:::bucket/cat.jpg"},
		}},
	}
	if !Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("should allow")
	}
}

func TestDenyOverridesAllow(t *testing.T) {
	p := &Policy{
		Statement: []Statement{
			{
				Effect:   EffectAllow,
				Action:   []string{"s3:*"},
				Resource: []string{"arn:aws:s3:::bucket/*"},
			},
			{
				Effect:   EffectDeny,
				Action:   []string{"s3:DeleteObject"},
				Resource: []string{"arn:aws:s3:::bucket/*"},
			},
		},
	}
	if Evaluate([]*Policy{p}, "s3:DeleteObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("deny should win")
	}
}

func TestImplicitDeny(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"arn:aws:s3:::bucket/*"},
		}},
	}
	if Evaluate([]*Policy{p}, "s3:PutObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("should be implicit deny")
	}
}

func TestActionWildcard(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:*"},
			Resource: []string{"*"},
		}},
	}
	for _, action := range []string{"s3:GetObject", "s3:PutObject", "s3:ListBucket"} {
		if !Evaluate([]*Policy{p}, action, "arn:aws:s3:::bucket/cat.jpg") {
			t.Fatalf("s3:* should match %s", action)
		}
	}
}

func TestActionPrefixWildcard(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:Get*"},
			Resource: []string{"*"},
		}},
	}
	if !Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::bucket/x") {
		t.Fatal("s3:Get* should match s3:GetObject")
	}
	if Evaluate([]*Policy{p}, "s3:PutObject", "arn:aws:s3:::bucket/x") {
		t.Fatal("s3:Get* should NOT match s3:PutObject")
	}
}

func TestResourceWildcard(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"arn:aws:s3:::bucket/*"},
		}},
	}
	if !Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::bucket/folder/sub/cat.jpg") {
		t.Fatal("bucket/* should match nested path")
	}
	if Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::other-bucket/cat.jpg") {
		t.Fatal("should not match other bucket")
	}
}

func TestResourceExact(t *testing.T) {
	p := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"arn:aws:s3:::bucket/cat.jpg"},
		}},
	}
	if !Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("exact match should work")
	}
	if Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::bucket/dog.jpg") {
		t.Fatal("should not match different key")
	}
}

func TestMultipleStatementsAllowDeny(t *testing.T) {
	p := &Policy{
		Statement: []Statement{
			{
				Effect:   EffectAllow,
				Action:   []string{"s3:*"},
				Resource: []string{"arn:aws:s3:::photos/*"},
			},
			{
				Effect:   EffectDeny,
				Action:   []string{"s3:DeleteObject"},
				Resource: []string{"arn:aws:s3:::photos/secret/*"},
			},
		},
	}
	if !Evaluate([]*Policy{p}, "s3:GetObject", "arn:aws:s3:::photos/cat.jpg") {
		t.Fatal("get should be allowed")
	}
	if Evaluate([]*Policy{p}, "s3:DeleteObject", "arn:aws:s3:::photos/secret/key") {
		t.Fatal("delete in secret should be denied")
	}
	if !Evaluate([]*Policy{p}, "s3:DeleteObject", "arn:aws:s3:::photos/cat.jpg") {
		t.Fatal("delete outside secret should be allowed")
	}
}

func TestMultiplePoliciesDenyWins(t *testing.T) {
	allowAll := &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:*"},
			Resource: []string{"*"},
		}},
	}
	denyDelete := &Policy{
		Statement: []Statement{{
			Effect:   EffectDeny,
			Action:   []string{"s3:DeleteObject"},
			Resource: []string{"arn:aws:s3:::bucket/*"},
		}},
	}
	if Evaluate([]*Policy{allowAll, denyDelete}, "s3:DeleteObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("deny in second policy should win")
	}
	if !Evaluate([]*Policy{allowAll, denyDelete}, "s3:GetObject", "arn:aws:s3:::bucket/cat.jpg") {
		t.Fatal("get should be allowed")
	}
}

func TestRootPolicyMatchesEverything(t *testing.T) {
	actions := []string{"s3:GetObject", "s3:PutObject", "s3:ListBucket", "admin:CreateUser"}
	resources := []string{"arn:aws:s3:::bucket/x", "arn:aws:s3:::any", "*"}
	for _, a := range actions {
		for _, r := range resources {
			if !Evaluate([]*Policy{RootPolicy}, a, r) {
				t.Fatalf("root policy should match %s / %s", a, r)
			}
		}
	}
}

func TestStoreCRUD(t *testing.T) {
	s := newTestStore(t)

	p := &Policy{
		Version: "2012-10-17",
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"*"},
		}},
	}
	if err := s.Put("readonly", p); err != nil {
		t.Fatal(err)
	}

	got, err := s.Get("readonly")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "readonly" {
		t.Fatalf("name: got %q, want %q", got.Name, "readonly")
	}
	if len(got.Statement) != 1 {
		t.Fatalf("statements: got %d", len(got.Statement))
	}

	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list: want 1, got %d", len(list))
	}

	if err := s.Delete("readonly"); err != nil {
		t.Fatal(err)
	}
	_, err = s.Get("readonly")
	if err != ErrPolicyNotFound {
		t.Fatalf("want ErrPolicyNotFound, got %v", err)
	}
}

func TestEvaluateByNames(t *testing.T) {
	s := newTestStore(t)

	s.Put("readonly", &Policy{
		Statement: []Statement{{
			Effect:   EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"*"},
		}},
	})
	s.Put("deny-all", &Policy{
		Statement: []Statement{{
			Effect:   EffectDeny,
			Action:   []string{"s3:*"},
			Resource: []string{"*"},
		}},
	})

	ok, err := s.EvaluateByNames([]string{"readonly"}, "s3:GetObject", "arn:aws:s3:::bucket/x")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("readonly should allow get")
	}

	ok, err = s.EvaluateByNames([]string{"readonly", "deny-all"}, "s3:GetObject", "arn:aws:s3:::bucket/x")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("deny-all should override")
	}

	ok, _ = s.EvaluateByNames([]string{"nonexistent"}, "s3:GetObject", "*")
	if ok {
		t.Fatal("nonexistent policy should be implicit deny")
	}
}