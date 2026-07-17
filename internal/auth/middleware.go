package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Zaki-goumri/vexo/internal/iam"
	"github.com/Zaki-goumri/vexo/internal/policy"
)

type ctxKey string

const userCtxKey ctxKey = "vexo.user"

var (
	ErrNoAuthHeader      = errors.New("missing Authorization header")
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrAccessKeyDisabled = errors.New("access key disabled")
	ErrAccessDenied      = errors.New("access denied")
)

type Middleware struct {
	IAM    *iam.Store
	Policy *policy.Store
	Next   http.Handler
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeAuthError(w, r, ErrNoAuthHeader)
		return
	}

	parsed, err := ParseAuthHeader(authHeader)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}

	ak, err := m.IAM.GetAccessKey(parsed.AccessKey)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	if ak.Status != iam.StatusActive {
		writeAuthError(w, r, ErrAccessKeyDisabled)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	expectedSig := parsed.Signature
	timestamp := r.Header.Get("x-amz-date")
	if timestamp == "" {
		timestamp = parsed.Date
	}

	signedHeadersList := strings.Split(parsed.SignedHeaders, ";")
	filteredHeaders := http.Header{}
	for _, sh := range signedHeadersList {
		sh = strings.ToLower(sh)
		if sh == "host" {
			filteredHeaders.Set("Host", r.Host)
			continue
		}
		for k, v := range r.Header {
			if strings.ToLower(k) == sh {
				filteredHeaders[k] = v
			}
		}
	}

	host := r.Host
	uri := r.URL.Path
	query := canonicalQueryFromURL(r)

	ok, err := VerifySignature(VerifyParams{
		Secret:    ak.PlainSecret,
		Method:    r.Method,
		URI:       uri,
		Host:      host,
		Query:     query,
		Headers:   filteredHeaders,
		Payload:   body,
		Timestamp: timestamp,
		Region:    parsed.Region,
		Service:   parsed.Service,
	}, expectedSig)
	if err != nil || !ok {
		writeAuthError(w, r, ErrInvalidSignature)
		return
	}

	action, resource := deriveActionAndResource(r)

	user, err := m.IAM.GetUser(ak.Username)
	if err != nil {
		writeAuthError(w, r, err)
		return
	}

	policyNames := append([]string{}, user.Policies...)
	for _, g := range user.Groups {
		grp, err := m.IAM.GetGroup(g)
		if err != nil {
			continue
		}
		policyNames = append(policyNames, grp.Policies...)
	}

	allowed, err := m.Policy.EvaluateByNames(policyNames, action, resource)
	if err != nil || !allowed {
		writeAuthError(w, r, ErrAccessDenied)
		return
	}

	ctx := context.WithValue(r.Context(), userCtxKey, ak.Username)
	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

func canonicalQueryFromURL(r *http.Request) string {
	return canonicalQuery(r.URL.RawQuery)
}

func deriveActionAndResource(r *http.Request) (string, string) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		return "s3:ListAllMyBuckets", "*"
	}

	idx := strings.Index(path, "/")
	if idx < 0 {
		var action string
		switch r.Method {
		case http.MethodPut:
			action = "s3:CreateBucket"
		case http.MethodDelete:
			action = "s3:DeleteBucket"
		case http.MethodHead:
			action = "s3:ListBucket"
		case http.MethodGet:
			action = "s3:ListBucket"
		}
		return action, "arn:aws:s3:::" + path
	}

	bucket := path[:idx]
	key := path[idx+1:]
	var action string
	switch r.Method {
	case http.MethodPut:
		action = "s3:PutObject"
	case http.MethodGet:
		action = "s3:GetObject"
	case http.MethodHead:
		action = "s3:GetObject"
	case http.MethodDelete:
		action = "s3:DeleteObject"
	}
	return action, "arn:aws:s3:::" + bucket + "/" + key
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write([]byte(`<Error><Code>AccessDenied</Code><Message>` + err.Error() + `</Message><Resource>` + r.URL.Path + `</Resource></Error>`))
}

func UsernameFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(userCtxKey).(string); ok {
		return v
	}
	return ""
}

func (m *Middleware) Handler() http.Handler {
	return http.HandlerFunc(m.ServeHTTP)
}