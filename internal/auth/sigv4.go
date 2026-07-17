package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const (
	algorithm  = "AWS4-HMAC-SHA256"
	serviceS3  = "s3"
	regionDef  = "us-east-1"
)

type Signer struct {
	AccessKey string
	Secret    string
	Region    string
	Service   string
}

type SignedRequest struct {
	Authorization string
	SignedHeaders string
	Signature     string
	Timestamp     string
}

func (s *Signer) sign(method, uri, query, host string, headers http.Header, payload []byte, timestamp string) (*SignedRequest, error) {
	if s.Region == "" {
		s.Region = regionDef
	}
	if s.Service == "" {
		s.Service = serviceS3
	}

	signedHeaders, canonicalHeaders := buildCanonicalHeaders(headers)

	payloadHash := hex.EncodeToString(sha256Sum(payload))

	canonicalReq := buildCanonicalRequest(method, uri, query, canonicalHeaders, signedHeaders, payloadHash)
	canonicalHash := hex.EncodeToString(sha256Sum([]byte(canonicalReq)))

	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", timestamp[:8], s.Region, s.Service)
	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s", algorithm, timestamp, credentialScope, canonicalHash)

	signingKey := buildSigningKey(s.Secret, timestamp[:8], s.Region, s.Service)
	signature := hex.EncodeToString(hmacSha256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.AccessKey, credentialScope, signedHeaders, signature)

	return &SignedRequest{
		Authorization: authHeader,
		SignedHeaders: signedHeaders,
		Signature:     signature,
		Timestamp:     timestamp,
	}, nil
}

func buildCanonicalRequest(method, uri, query, canonicalHeaders, signedHeaders, payloadHash string) string {
	return strings.Join([]string{
		method,
		uri,
		query,
		canonicalHeaders + "\n",
		signedHeaders,
		payloadHash,
	}, "\n")
}

func buildCanonicalHeaders(headers http.Header) (signedHeaders string, canonicalHeaders string) {
	var keys []string
	hdrCopy := make(http.Header)
	for k, v := range headers {
		lk := strings.ToLower(k)
		hdrCopy[lk] = v
		keys = append(keys, lk)
	}
	sort.Strings(keys)

	var canParts []string
	for _, k := range keys {
		val := strings.TrimSpace(strings.Join(hdrCopy[k], ","))
		canParts = append(canParts, k+":"+val)
	}
	return strings.Join(keys, ";"), strings.Join(canParts, "\n")
}

func buildSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSha256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSha256(kDate, []byte(region))
	kService := hmacSha256(kRegion, []byte(service))
	return hmacSha256(kService, []byte("aws4_request"))
}

func hmacSha256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func sha256Sum(data []byte) []byte {
	h := sha256.New()
	h.Write(data)
	return h.Sum(nil)
}

type AuthHeader struct {
	AccessKey     string
	Date          string
	Region        string
	Service       string
	SignedHeaders string
	Signature     string
}

func ParseAuthHeader(authHeader string) (*AuthHeader, error) {
	if !strings.HasPrefix(authHeader, algorithm) {
		return nil, fmt.Errorf("unsupported algorithm")
	}
	rest := strings.TrimPrefix(authHeader, algorithm+" ")
	parts := strings.Split(rest, ", ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed auth header")
	}

	credPart := strings.TrimPrefix(parts[0], "Credential=")
	credFields := strings.Split(credPart, "/")
	if len(credFields) != 5 {
		return nil, fmt.Errorf("malformed credential scope")
	}

	return &AuthHeader{
		AccessKey:     credFields[0],
		Date:         credFields[1],
		Region:       credFields[2],
		Service:      credFields[3],
		SignedHeaders: strings.TrimPrefix(parts[1], "SignedHeaders="),
		Signature:     strings.TrimPrefix(parts[2], "Signature="),
	}, nil
}

type VerifyParams struct {
	Secret    string
	Method    string
	URI       string
	Host      string
	Query     string
	Headers   http.Header
	Payload   []byte
	Timestamp string
	Region    string
	Service   string
}

func VerifySignature(params VerifyParams, clientSignature string) (bool, error) {
	if params.Region == "" {
		params.Region = regionDef
	}
	if params.Service == "" {
		params.Service = serviceS3
	}

	signer := &Signer{
		AccessKey: "",
		Secret:    params.Secret,
		Region:    params.Region,
		Service:   params.Service,
	}
	signed, err := signer.sign(params.Method, params.URI, params.Query, params.Host, params.Headers, params.Payload, params.Timestamp)
	if err != nil {
		return false, err
	}
	return hmac.Equal([]byte(signed.Signature), []byte(clientSignature)), nil
}

func canonicalQuery(query string) string {
	if query == "" {
		return ""
	}
	vals, err := url.ParseQuery(query)
	if err != nil {
		return query
	}
	var keys []string
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		enc := url.QueryEscape(k)
		enc = strings.ReplaceAll(enc, "+", "%20")
		val := strings.TrimSpace(vals.Get(k))
		parts = append(parts, enc+"="+url.QueryEscape(val))
	}
	return strings.Join(parts, "&")
}

func SignRequest(method, uri, rawQuery, host string, headers http.Header, payload []byte, accessKey, secret, timestamp string) (*SignedRequest, error) {
	s := &Signer{
		AccessKey: accessKey,
		Secret:    secret,
		Region:    regionDef,
		Service:   serviceS3,
	}
	return s.sign(method, uri, canonicalQuery(rawQuery), host, headers, payload, timestamp)
}