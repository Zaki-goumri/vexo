package scanner

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/lifecycle"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func newTestEnv(t *testing.T) (*Scanner, *buckets.Store, *storage.Store, *lifecycle.Store) {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	bucketStore := buckets.NewStore(meta, tmp)
	store := storage.NewStore(meta, bucketStore, tmp)
	lcStore := lifecycle.NewStore(bucketStore)
	sc := New(bucketStore, store, lcStore)
	return sc, bucketStore, store, lcStore
}

func setObjectTimes(t *testing.T, store *storage.Store, bucket, key string, created, lastAccess time.Time) {
	t.Helper()
	m, err := store.Stat(bucket, key)
	if err != nil {
		t.Fatal(err)
	}
	m.CreatedAt = created
	m.LastAccessedAt = lastAccess
	data, _ := json.Marshal(m)
	if err := store.Meta().Put("objects", bucket+"/"+key, data); err != nil {
		t.Fatal(err)
	}
}

func assertTier(t *testing.T, store *storage.Store, bucket, key, wantTier string) {
	t.Helper()
	m, err := store.Stat(bucket, key)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tier != wantTier {
		t.Fatalf("tier for %s/%s: got %q, want %q", bucket, key, m.Tier, wantTier)
	}
}

func assertGone(t *testing.T, store *storage.Store, bucket, key string) {
	t.Helper()
	_, err := store.Stat(bucket, key)
	if err != storage.ErrObjectNotFound {
		t.Fatalf("want ErrObjectNotFound for %s/%s, got %v", bucket, key, err)
	}
}

func TestScanTransitionsOldHotToCold(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "old.jpg", readCloser("data"))

	past := time.Now().AddDate(0, 0, -45)
	setObjectTimes(t, store, "photos", "old.jpg", past, past)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	})

	transitioned, deleted, err := sc.ScanOnce()
	if err != nil {
		t.Fatal(err)
	}
	if transitioned != 1 {
		t.Fatalf("transitioned: want 1, got %d", transitioned)
	}
	if deleted != 0 {
		t.Fatalf("deleted: want 0, got %d", deleted)
	}
	assertTier(t, store, "photos", "old.jpg", storage.TierCold)
}

func TestScanRecentlyAccessedNoAction(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "fresh.jpg", readCloser("data"))

	now := time.Now()
	setObjectTimes(t, store, "photos", "fresh.jpg", now, now)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	})

	transitioned, deleted, _ := sc.ScanOnce()
	if transitioned != 0 || deleted != 0 {
		t.Fatalf("want no action, got transitioned=%d deleted=%d", transitioned, deleted)
	}
	assertTier(t, store, "photos", "fresh.jpg", storage.TierHot)
}

func TestScanExpiresOldObject(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "ancient.jpg", readClocer("data"))

	past := time.Now().AddDate(0, 0, -400)
	setObjectTimes(t, store, "photos", "ancient.jpg", past, past)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:         "r1",
			Status:     lifecycle.StatusEnabled,
			Expiration: &lifecycle.Expiration{Days: 365},
		}},
	})

	transitioned, deleted, _ := sc.ScanOnce()
	if transitioned != 0 {
		t.Fatalf("transitioned: want 0, got %d", transitioned)
	}
	if deleted != 1 {
		t.Fatalf("deleted: want 1, got %d", deleted)
	}
	assertGone(t, store, "photos", "ancient.jpg")
}

func TestScanNoLifecycleNoAction(t *testing.T) {
	sc, bucketStore, store, _ := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "cat.jpg", readCloser("data"))

	past := time.Now().AddDate(0, 0, -100)
	setObjectTimes(t, store, "photos", "cat.jpg", past, past)

	transitioned, deleted, _ := sc.ScanOnce()
	if transitioned != 0 || deleted != 0 {
		t.Fatalf("want no action without lifecycle, got t=%d d=%d", transitioned, deleted)
	}
}

func TestScanMixedObjects(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")

	store.Put("photos", "fresh.jpg", readCloser("a"))
	store.Put("photos", "old.jpg", readCloser("b"))
	store.Put("photos", "expired.jpg", readClocer("c"))

	now := time.Now()
	setObjectTimes(t, store, "photos", "fresh.jpg", now, now)
	old := now.AddDate(0, 0, -50)
	setObjectTimes(t, store, "photos", "old.jpg", old, old)
	expired := now.AddDate(0, 0, -400)
	setObjectTimes(t, store, "photos", "expired.jpg", expired, expired)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 30, Tier: storage.TierCold}},
			Expiration:  &lifecycle.Expiration{Days: 365},
		}},
	})

	transitioned, deleted, _ := sc.ScanOnce()
	if transitioned != 1 {
		t.Fatalf("transitioned: want 1, got %d", transitioned)
	}
	if deleted != 1 {
		t.Fatalf("deleted: want 1, got %d", deleted)
	}
	assertTier(t, store, "photos", "fresh.jpg", storage.TierHot)
	assertTier(t, store, "photos", "old.jpg", storage.TierCold)
	assertGone(t, store, "photos", "expired.jpg")
}

func TestScanInfrequentToCold(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "doc.jpg", readCloser("data"))

	meta, _ := store.Stat("photos", "doc.jpg")
	store.Transition("photos", "doc.jpg", storage.TierInfrequent)

	past := time.Now().AddDate(0, 0, -100)
	setObjectTimes(t, store, "photos", "doc.jpg", past, past)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 90, Tier: storage.TierCold}},
		}},
	})

	transitioned, _, _ := sc.ScanOnce()
	if transitioned != 1 {
		t.Fatalf("transitioned: want 1, got %d", transitioned)
	}
	assertTier(t, store, "photos", "doc.jpg", storage.TierCold)
	_ = meta
}

func TestScanMultipleBuckets(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("bucket-a")
	bucketStore.Create("bucket-b")
	bucketStore.Create("bucket-c")

	store.Put("bucket-a", "old.jpg", readCloser("a"))
	store.Put("bucket-c", "old.jpg", readCloser("c"))

	past := time.Now().AddDate(0, 0, -50)
	setObjectTimes(t, store, "bucket-a", "old.jpg", past, past)
	setObjectTimes(t, store, "bucket-c", "old.jpg", past, past)

	cfg := &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	}
	lcStore.SetLifecycle("bucket-a", cfg)
	lcStore.SetLifecycle("bucket-c", cfg)

	transitioned, _, _ := sc.ScanOnce()
	if transitioned != 2 {
		t.Fatalf("transitioned: want 2, got %d", transitioned)
	}
	assertTier(t, store, "bucket-a", "old.jpg", storage.TierCold)
	assertTier(t, store, "bucket-c", "old.jpg", storage.TierCold)
}

func TestStartStopScanner(t *testing.T) {
	sc, bucketStore, store, lcStore := newTestEnv(t)
	bucketStore.Create("photos")
	store.Put("photos", "old.jpg", readCloser("data"))

	past := time.Now().AddDate(0, 0, -50)
	setObjectTimes(t, store, "photos", "old.jpg", past, past)

	lcStore.SetLifecycle("photos", &lifecycle.Config{
		Rules: []lifecycle.Rule{{
			ID:          "r1",
			Status:      lifecycle.StatusEnabled,
			Transitions: []lifecycle.Transition{{Days: 30, Tier: storage.TierCold}},
		}},
	})

	sc.WithInterval(100 * time.Millisecond)
	sc.Start()
	time.Sleep(300 * time.Millisecond)
	sc.Stop()

	assertTier(t, store, "photos", "old.jpg", storage.TierCold)
}

func readCloser(s string) *bytes.Reader { return bytes.NewReader([]byte(s)) }
func readClocer(s string) *bytes.Reader { return bytes.NewReader([]byte(s)) }