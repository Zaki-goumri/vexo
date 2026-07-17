package auth

import (
	"net/http"
	"testing"
	"time"
)

func TestSignAndVerifyRoundtrip(t *testing.T) {
	accessKey := "AKID1234567890ABCDEF"
	secret := "secretkey123"
	timestamp := time.Now().UTC().Format("20060102T150405Z")

	headers := http.Header{}
	headers.Set("Host", "localhost:9000")
	headers.Set("x-amz-date", timestamp)
	headers.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")

	signed, err := SignRequest("PUT", "/test-bkt/cat.jpg", "", "localhost:9000", headers, []byte("hello"), accessKey, secret, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Authorization == "" {
		t.Fatal("authorization header is empty")
	}

	parsed, err := ParseAuthHeader(signed.Authorization)
	if err != nil {
		t.Fatalf("parse auth: %v", err)
	}
	if parsed.AccessKey != accessKey {
		t.Fatalf("access key: got %q, want %q", parsed.AccessKey, accessKey)
	}

	ok, err := VerifySignature(VerifyParams{
		Secret:    secret,
		Method:    "PUT",
		URI:       "/test-bkt/cat.jpg",
		Host:      "localhost:9000",
		Query:     "",
		Headers:   headers,
		Payload:   []byte("hello"),
		Timestamp: timestamp,
	}, parsed.Signature)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("signature should verify")
	}
}

func TestTamperedPayloadFails(t *testing.T) {
	secret := "secretkey123"
	timestamp := "20260717T150405Z"
	headers := http.Header{}
	headers.Set("Host", "localhost:9000")
	headers.Set("x-amz-date", timestamp)

	payload := []byte("hello")
	signed, err := SignRequest("PUT", "/bkt/cat.jpg", "", "localhost:9000", headers, payload, "AKID", secret, timestamp)
	if err != nil {
		t.Fatal(err)
	}

	parsed, _ := ParseAuthHeader(signed.Authorization)

	tamperedHeaders := http.Header{}
	tamperedHeaders.Set("Host", "localhost:9000")
	tamperedHeaders.Set("x-amz-date", timestamp)

	ok, _ := VerifySignature(VerifyParams{
		Secret:    secret,
		Method:    "PUT",
		URI:       "/bkt/cat.jpg",
		Host:      "localhost:9000",
		Query:     "",
		Headers:   tamperedHeaders,
		Payload:   []byte("TAMPERED"),
		Timestamp: timestamp,
	}, parsed.Signature)
	if ok {
		t.Fatal("tampered payload should not verify")
	}
}

func TestTamperedMethodFails(t *testing.T) {
	secret := "secretkey123"
	timestamp := "20260717T150405Z"
	headers := http.Header{}
	headers.Set("Host", "localhost:9000")
	headers.Set("x-amz-date", timestamp)

	signed, err := SignRequest("GET", "/bkt/cat.jpg", "", "localhost:9000", headers, []byte("hello"), "AKID", secret, timestamp)
	if err != nil {
		t.Fatal(err)
	}

	parsed, _ := ParseAuthHeader(signed.Authorization)

	ok, _ := VerifySignature(VerifyParams{
		Secret:    secret,
		Method:    "DELETE",
		URI:       "/bkt/cat.jpg",
		Host:      "localhost:9000",
		Query:     "",
		Headers:   headers,
		Payload:   []byte("hello"),
		Timestamp: timestamp,
	}, parsed.Signature)
	if ok {
		t.Fatal("tampered method should not verify")
	}
}

func TestWrongSecretFails(t *testing.T) {
	secret := "secretkey123"
	timestamp := "20260717T150405Z"
	headers := http.Header{}
	headers.Set("Host", "localhost:9000")
	headers.Set("x-amz-date", timestamp)

	signed, err := SignRequest("PUT", "/bkt/cat.jpg", "", "localhost:9000", headers, []byte("hello"), "AKID", secret, timestamp)
	if err != nil {
		t.Fatal(err)
	}

	parsed, _ := ParseAuthHeader(signed.Authorization)

	ok, _ := VerifySignature(VerifyParams{
		Secret:    "wrong-secret",
		Method:    "PUT",
		URI:       "/bkt/cat.jpg",
		Host:      "localhost:9000",
		Query:     "",
		Headers:   headers,
		Payload:   []byte("hello"),
		Timestamp: timestamp,
	}, parsed.Signature)
	if ok {
		t.Fatal("wrong secret should not verify")
	}
}

func TestParseAuthHeaderMalformed(t *testing.T) {
	_, err := ParseAuthHeader("Basic abc123")
	if err == nil {
		t.Fatal("should fail on non-AWS4 algorithm")
	}

	_, err = ParseAuthHeader("AWS4-HMAC-SHA256 garbage")
	if err == nil {
		t.Fatal("should fail on malformed header")
	}
}