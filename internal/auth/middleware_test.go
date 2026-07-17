package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/iam"
	"github.com/Zaki-goumri/vexo/internal/policy"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func signReq(t *testing.T, method, path, host, accessKey, secret string, payload []byte) *http.Request {
	t.Helper()
	timestamp := time.Now().UTC().Format("20060102T150405Z")

	headers := http.Header{}
	headers.Set("Host", host)
	headers.Set("x-amz-date", timestamp)

	if payload != nil {
		headers.Set("x-amz-content-sha256", sha256Hex(payload))
	} else {
		headers.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")
	}

	signed, err := SignRequest(method, path, "", host, headers, payload, accessKey, secret, timestamp)
	if err != nil {
		t.Fatal(err)
	}

	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	} else {
		body = nil
	}
	req, _ := http.NewRequest(method, "http://"+host+path, body)
	req.Header = headers
	req.Header.Set("Authorization", signed.Authorization)
	return req
}

func newAuthTestServer(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })

	iamStore := iam.NewStore(meta)
	policyStore := policy.NewStore(meta)
	bucketStore := buckets.NewStore(meta, tmp)
	store := storage.NewStore(meta, bucketStore, tmp)

	policyStore.Put("root", policy.RootPolicy)

	u, err := iamStore.CreateUser("testuser")
	if err != nil {
		t.Fatal(err)
	}
	u.Policies = []string{"root"}
	data, _ := json.Marshal(u)
	meta.Put("users", "testuser", data)

	_, secret, err := iamStore.CreateAccessKey("testuser")
	if err != nil {
		t.Fatal(err)
	}
	keys, _ := iamStore.ListAccessKeys("testuser")
	accessKeyID := keys[0].AccessKey

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /{bucket}", func(w http.ResponseWriter, r *http.Request) {
		bucketStore.Create(r.PathValue("bucket"))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("PUT /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		store.Put(r.PathValue("bucket"), r.PathValue("key"), bytes.NewReader(body))
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		rc, _, err := store.Get(r.PathValue("bucket"), strings.TrimPrefix(r.PathValue("key"), "/"))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer rc.Close()
		io.Copy(w, rc)
	})

	mw := &Middleware{
		IAM:    iamStore,
		Policy: policyStore,
		Next:   mux,
	}

	ts := httptest.NewServer(http.HandlerFunc(mw.ServeHTTP))
	t.Cleanup(func() { ts.Close() })

	return ts, accessKeyID, secret
}

func TestValidSignatureAllowed(t *testing.T) {
	ts, ak, secret := newAuthTestServer(t)

	payload := []byte("hello vexo")
	req := signReq(t, "PUT", "/test-bkt/cat.jpg", ts.Listener.Addr().String(), ak, secret, payload)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

func TestInvalidSignatureDenied(t *testing.T) {
	ts, ak, _ := newAuthTestServer(t)

	payload := []byte("hello vexo")
	req := signReq(t, "PUT", "/test-bkt/cat.jpg", ts.Listener.Addr().String(), ak, "wrong-secret", payload)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestNoAuthHeaderDenied(t *testing.T) {
	ts, _, _ := newAuthTestServer(t)

	req, _ := http.NewRequest("PUT", "http://"+ts.Listener.Addr().String()+"/test-bkt/cat.jpg", strings.NewReader("data"))
	req.Header.Set("Host", ts.Listener.Addr().String())

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestPolicyDeny(t *testing.T) {
	tmp := t.TempDir()
	meta := &db.DB{}
	meta.Open(filepath.Join(tmp, "test.db"))
	t.Cleanup(func() { meta.Close() })

	iamStore := iam.NewStore(meta)
	policyStore := policy.NewStore(meta)
	bucketStore := buckets.NewStore(meta, tmp)
	store := storage.NewStore(meta, bucketStore, tmp)

	policyStore.Put("readonly", &policy.Policy{
		Statement: []policy.Statement{{
			Effect:   policy.EffectAllow,
			Action:   []string{"s3:GetObject"},
			Resource: []string{"*"},
		}},
	})

	u, _ := iamStore.CreateUser("readonly-user")
	u.Policies = []string{"readonly"}
	data, _ := json.Marshal(u)
	meta.Put("users", "readonly-user", data)

	_, secret, _ := iamStore.CreateAccessKey("readonly-user")
	keys, _ := iamStore.ListAccessKeys("readonly-user")
	ak := keys[0].AccessKey

	bucketStore.Create("test-bkt")
	store.Put("test-bkt", "cat.jpg", strings.NewReader("data"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		rc, _, err := store.Get(r.PathValue("bucket"), strings.TrimPrefix(r.PathValue("key"), "/"))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		defer rc.Close()
		io.Copy(w, rc)
	})
	mux.HandleFunc("PUT /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		store.Put(r.PathValue("bucket"), r.PathValue("key"), bytes.NewReader(body))
		w.WriteHeader(http.StatusOK)
	})

	mw := &Middleware{
		IAM:    iamStore,
		Policy: policyStore,
		Next:   mux,
	}
	ts := httptest.NewServer(http.HandlerFunc(mw.ServeHTTP))
	t.Cleanup(func() { ts.Close() })

	req := signReq(t, "GET", "/test-bkt/cat.jpg", ts.Listener.Addr().String(), ak, secret, nil)
	resp, _ := ts.Client().Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get should be allowed, got %d", resp.StatusCode)
	}

	req = signReq(t, "PUT", "/test-bkt/dog.jpg", ts.Listener.Addr().String(), ak, secret, []byte("data"))
	resp, _ = ts.Client().Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("put should be denied, got %d", resp.StatusCode)
	}
}