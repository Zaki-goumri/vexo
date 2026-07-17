package lifecycle

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func newTestStore(t *testing.T) (*Store, *buckets.Store, *storage.Store) {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	bucketStore := buckets.NewStore(meta, tmp)
	store := storage.NewStore(meta, bucketStore, tmp)
	return NewStore(bucketStore), bucketStore, store
}

func makeMeta(key, tier string, created, lastAccess time.Time) *storage.ObjectMeta {
	return &storage.ObjectMeta{
		Key:            key,
		Tier:           tier,
		CreatedAt:      created,
		LastAccessedAt: lastAccess,
	}
}

func TestSetGetLifecycle(t *testing.T) {
	lcStore, bucketStore, _ := newTestStore(t)
	bucketStore.Create("test-bkt")

	cfg := &Config{
		Rules: []Rule{{
			ID:     "rule1",
			Status: StatusEnabled,
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}
	if err := lcStore.SetLifecycle("test-bkt", cfg); err != nil {
		t.Fatal(err)
	}

	got, err := lcStore.GetLifecycle("test-bkt")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(got.Rules))
	}
	if got.Rules[0].Transitions[0].Days != 30 {
		t.Fatalf("days: got %d", got.Rules[0].Transitions[0].Days)
	}
}

func TestGetNoLifecycle(t *testing.T) {
	lcStore, bucketStore, _ := newTestStore(t)
	bucketStore.Create("test-bkt")

	_, err := lcStore.GetLifecycle("test-bkt")
	if err != ErrNoLifecycle {
		t.Fatalf("want ErrNoLifecycle, got %v", err)
	}
}

func TestDeleteLifecycle(t *testing.T) {
	lcStore, bucketStore, _ := newTestStore(t)
	bucketStore.Create("test-bkt")

	cfg := &Config{Rules: []Rule{{ID: "r1", Status: StatusEnabled}}}
	lcStore.SetLifecycle("test-bkt", cfg)

	if err := lcStore.DeleteLifecycle("test-bkt"); err != nil {
		t.Fatal(err)
	}

	_, err := lcStore.GetLifecycle("test-bkt")
	if err != ErrNoLifecycle {
		t.Fatalf("want ErrNoLifecycle after delete, got %v", err)
	}
}

func TestNoRulesNoAction(t *testing.T) {
	now := time.Now()
	meta := makeMeta("cat.jpg", storage.TierHot, now, now)

	cfg := &Config{}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionNone {
		t.Fatalf("want none, got %s", action)
	}
}

func TestOldObjectTransitionsToCold(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -45)
	created := now.AddDate(0, 0, -60)
	meta := makeMeta("cat.jpg", storage.TierHot, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:     "r1",
			Status: StatusEnabled,
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}
	action, tier := cfg.Evaluate(meta, now)
	if action != ActionTransition {
		t.Fatalf("want transition, got %s", action)
	}
	if tier != storage.TierCold {
		t.Fatalf("want cold, got %s", tier)
	}
}

func TestRecentlyAccessedNoAction(t *testing.T) {
	now := time.Now()
	recent := now.AddDate(0, 0, -5)
	created := now.AddDate(0, 0, -60)
	meta := makeMeta("cat.jpg", storage.TierHot, created, recent)

	cfg := &Config{
		Rules: []Rule{{
			ID:     "r1",
			Status: StatusEnabled,
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionNone {
		t.Fatalf("want none, got %s", action)
	}
}

func TestExpiredObjectDeleted(t *testing.T) {
	now := time.Now()
	created := now.AddDate(0, 0, -400)
	meta := makeMeta("cat.jpg", storage.TierHot, created, created)

	cfg := &Config{
		Rules: []Rule{{
			ID:         "r1",
			Status:     StatusEnabled,
			Expiration: &Expiration{Days: 365},
		}},
	}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionDelete {
		t.Fatalf("want delete, got %s", action)
	}
}

func TestDeleteWinsOverTransition(t *testing.T) {
	now := time.Now()
	created := now.AddDate(0, 0, -400)
	oldAccess := now.AddDate(0, 0, -100)
	meta := makeMeta("cat.jpg", storage.TierHot, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:          "r1",
			Status:      StatusEnabled,
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
			Expiration:  &Expiration{Days: 365},
		}},
	}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionDelete {
		t.Fatalf("delete should win, got %s", action)
	}
}

func TestDisabledRuleNoAction(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -90)
	created := now.AddDate(0, 0, -90)
	meta := makeMeta("cat.jpg", storage.TierHot, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:          "r1",
			Status:      StatusDisabled,
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionNone {
		t.Fatalf("disabled rule should be skipped, got %s", action)
	}
}

func TestPrefixFilter(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -60)
	created := now.AddDate(0, 0, -60)

	cfg := &Config{
		Rules: []Rule{{
			ID:          "r1",
			Status:      StatusEnabled,
			Prefix:      "logs/",
			Transitions: []Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}

	meta1 := makeMeta("cat.jpg", storage.TierHot, created, oldAccess)
	action, _ := cfg.Evaluate(meta1, now)
	if action != ActionNone {
		t.Fatal("non-matching prefix should not trigger")
	}

	meta2 := makeMeta("logs/app.log", storage.TierHot, created, oldAccess)
	action, tier := cfg.Evaluate(meta2, now)
	if action != ActionTransition || tier != storage.TierCold {
		t.Fatalf("matching prefix should transition, got %s/%s", action, tier)
	}
}

func TestMultipleTransitionsPickColdest(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -100)
	created := now.AddDate(0, 0, -100)
	meta := makeMeta("cat.jpg", storage.TierHot, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:     "r1",
			Status: StatusEnabled,
			Transitions: []Transition{
				{Days: 30, Tier: storage.TierInfrequent},
				{Days: 90, Tier: storage.TierCold},
			},
		}},
	}
	action, tier := cfg.Evaluate(meta, now)
	if action != ActionTransition {
		t.Fatalf("want transition, got %s", action)
	}
	if tier != storage.TierCold {
		t.Fatalf("want coldest (cold), got %s", tier)
	}
}

func TestAlreadyAtColdNoTransition(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -100)
	created := now.AddDate(0, 0, -100)
	meta := makeMeta("cat.jpg", storage.TierCold, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:     "r1",
			Status: StatusEnabled,
			Transitions: []Transition{
				{Days: 30, Tier: storage.TierInfrequent},
				{Days: 90, Tier: storage.TierCold},
			},
		}},
	}
	action, _ := cfg.Evaluate(meta, now)
	if action != ActionNone {
		t.Fatalf("already cold should not transition, got %s", action)
	}
}

func TestInfrequentToCold(t *testing.T) {
	now := time.Now()
	oldAccess := now.AddDate(0, 0, -100)
	created := now.AddDate(0, 0, -200)
	meta := makeMeta("cat.jpg", storage.TierInfrequent, created, oldAccess)

	cfg := &Config{
		Rules: []Rule{{
			ID:     "r1",
			Status: StatusEnabled,
			Transitions: []Transition{{Days: 90, Tier: storage.TierCold}},
		}},
	}
	action, tier := cfg.Evaluate(meta, now)
	if action != ActionTransition {
		t.Fatalf("want transition, got %s", action)
	}
	if tier != storage.TierCold {
		t.Fatalf("want cold, got %s", tier)
	}
}