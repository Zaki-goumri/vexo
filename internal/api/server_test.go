package api

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/db"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	tmp := t.TempDir()
	meta := &db.DB{}
	if err := meta.Open(filepath.Join(tmp, "test.db")); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { meta.Close() })

	bucketStore := buckets.NewStore(meta, tmp)
	store := storage.NewStore(meta, bucketStore, tmp)
	srv := NewServer(bucketStore, store, "")

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close() })

	return ts, srv
}

func do(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, b
}

func TestCreateAndListBuckets(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, _ := do(t, ts, "PUT", "/test-bkt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: want 200, got %d", resp.StatusCode)
	}

	resp, body := do(t, ts, "GET", "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d", resp.StatusCode)
	}

	var result ListAllMyBucketsResult
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range result.Buckets {
		if b.Name == "test-bkt" {
			found = true
		}
	}
	if !found {
		t.Fatal("bucket not found in listing")
	}
}

func TestCreateDuplicateBucket(t *testing.T) {
	ts, _ := newTestServer(t)

	do(t, ts, "PUT", "/test-bkt", nil)
	resp, body := do(t, ts, "PUT", "/test-bkt", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "BucketAlreadyExists") {
		t.Fatalf("want BucketAlreadyExists, got %s", body)
	}
}

func TestHeadBucket(t *testing.T) {
	ts, _ := newTestServer(t)

	do(t, ts, "PUT", "/test-bkt", nil)

	resp, _ := do(t, ts, "HEAD", "/test-bkt", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	resp, _ = do(t, ts, "HEAD", "/ghost", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestPutAndGetAndHeadObject(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	payload := []byte("hello vexo over http")
	resp, _ := do(t, ts, "PUT", "/test-bkt/cat.jpg", bytes.NewReader(payload))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put: want 200, got %d", resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("ETag header is empty")
	}

	resp, body := do(t, ts, "GET", "/test-bkt/cat.jpg", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get: want 200, got %d", resp.StatusCode)
	}
	if !bytes.Equal(body, payload) {
		t.Fatalf("body: want %q, got %q", payload, body)
	}
	if resp.Header.Get("ETag") != etag {
		t.Fatalf("etag mismatch: got %q, want %q", resp.Header.Get("ETag"), etag)
	}
	clen := resp.Header.Get("Content-Length")
	if clen != strconv.Itoa(len(payload)) {
		t.Fatalf("content-length: got %q, want %d", clen, len(payload))
	}

	resp, body = do(t, ts, "HEAD", "/test-bkt/cat.jpg", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("head: want 200, got %d", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Fatalf("head should have no body, got %d bytes", len(body))
	}
	if resp.Header.Get("ETag") != etag {
		t.Fatalf("head etag: got %q, want %q", resp.Header.Get("ETag"), etag)
	}
}

func TestGetMissingObject(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	resp, body := do(t, ts, "GET", "/test-bkt/ghost.jpg", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "NoSuchKey") {
		t.Fatalf("want NoSuchKey, got %s", body)
	}
}

func TestDeleteObject(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)
	do(t, ts, "PUT", "/test-bkt/cat.jpg", strings.NewReader("data"))

	resp, _ := do(t, ts, "DELETE", "/test-bkt/cat.jpg", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", resp.StatusCode)
	}

	resp, _ = do(t, ts, "GET", "/test-bkt/cat.jpg", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: want 404, got %d", resp.StatusCode)
	}
}

func TestListObjects(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	for _, key := range []string{"cat.jpg", "dog.jpg", "bird.png"} {
		do(t, ts, "PUT", "/test-bkt/"+key, strings.NewReader(key))
	}

	resp, body := do(t, ts, "GET", "/test-bkt?prefix=", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d", resp.StatusCode)
	}

	var result ListBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.KeyCount != 3 {
		t.Fatalf("KeyCount: want 3, got %d", result.KeyCount)
	}
	keys := map[string]bool{}
	for _, obj := range result.Contents {
		keys[obj.Key] = true
	}
	for _, k := range []string{"cat.jpg", "dog.jpg", "bird.png"} {
		if !keys[k] {
			t.Fatalf("missing key %q", k)
		}
	}
}

func TestListObjectsWithPrefix(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	for _, key := range []string{"photos/cat.jpg", "photos/dog.jpg", "docs/readme.txt"} {
		do(t, ts, "PUT", "/test-bkt/"+key, strings.NewReader(key))
	}

	resp, body := do(t, ts, "GET", "/test-bkt?prefix=photos/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: want 200, got %d", resp.StatusCode)
	}

	var result ListBucketResult
	if err := xml.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.KeyCount != 2 {
		t.Fatalf("KeyCount: want 2, got %d", result.KeyCount)
	}
}

func TestDeleteBucket(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	resp, _ := do(t, ts, "DELETE", "/test-bkt", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}

	resp, _ = do(t, ts, "HEAD", "/test-bkt", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("head after delete: want 404, got %d", resp.StatusCode)
	}
}

func TestDeleteBucketNotEmpty(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)
	do(t, ts, "PUT", "/test-bkt/cat.jpg", strings.NewReader("data"))

	resp, body := do(t, ts, "DELETE", "/test-bkt", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("want 409, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "BucketNotEmpty") {
		t.Fatalf("want BucketNotEmpty, got %s", body)
	}
}

func TestPutToMissingBucket(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, body := do(t, ts, "PUT", "/ghost/cat.jpg", strings.NewReader("data"))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "NoSuchBucket") {
		t.Fatalf("want NoSuchBucket, got %s", body)
	}
}

func TestNestedKeyPath(t *testing.T) {
	ts, _ := newTestServer(t)
	do(t, ts, "PUT", "/test-bkt", nil)

	payload := []byte("nested content")
	resp, _ := do(t, ts, "PUT", "/test-bkt/folder/sub/cat.jpg", bytes.NewReader(payload))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put nested: want 200, got %d", resp.StatusCode)
	}

	resp, body := do(t, ts, "GET", "/test-bkt/folder/sub/cat.jpg", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get nested: want 200, got %d", resp.StatusCode)
	}
	if !bytes.Equal(body, payload) {
		t.Fatalf("body: want %q, got %q", payload, body)
	}
}