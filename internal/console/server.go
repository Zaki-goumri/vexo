package console

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/iam"
	"github.com/Zaki-goumri/vexo/internal/policy"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

//go:embed all:web/dist
var staticFiles embed.FS

type Server struct {
	sessions    *SessionStore
	iam         *iam.Store
	policy      *policy.Store
	bucketStore *buckets.Store
	storage     *storage.Store
	shareSigner *ShareSigner
	addr        string
}

func NewServer(iamStore *iam.Store, policyStore *policy.Store, bucketStore *buckets.Store, store *storage.Store, shareSigner *ShareSigner, addr string) *Server {
	return &Server{
		sessions:    NewSessionStore(),
		iam:         iamStore,
		policy:      policyStore,
		bucketStore: bucketStore,
		storage:     store,
		shareSigner: shareSigner,
		addr:        addr,
	}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/whoami", s.handleWhoami)

	mux.Handle("GET /api/buckets", s.authMiddleware(http.HandlerFunc(s.handleListBuckets)))
	mux.Handle("POST /api/buckets", s.authMiddleware(http.HandlerFunc(s.handleCreateBucket)))
	mux.Handle("GET /api/buckets/{name}", s.authMiddleware(http.HandlerFunc(s.handleGetBucket)))
	mux.Handle("DELETE /api/buckets/{name}", s.authMiddleware(http.HandlerFunc(s.handleDeleteBucket)))
	mux.Handle("PATCH /api/buckets/{name}/visibility", s.authMiddleware(http.HandlerFunc(s.handleSetBucketVisibility)))

	mux.Handle("GET /api/buckets/{name}/objects", s.authMiddleware(http.HandlerFunc(s.handleListObjects)))
	mux.Handle("POST /api/buckets/{name}/objects/rename", s.authMiddleware(http.HandlerFunc(s.handleRenameObject)))
	mux.Handle("POST /api/buckets/{name}/objects/share", s.authMiddleware(http.HandlerFunc(s.handleShareObject)))
	mux.Handle("PUT /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handlePutObject)))
	mux.Handle("GET /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handleGetObject)))
	mux.Handle("DELETE /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handleDeleteObject)))

	mux.HandleFunc("GET /api/public/{bucket}/{key...}", s.handlePublicGetObject)

	mux.Handle("GET /api/users", s.authMiddleware(http.HandlerFunc(s.handleListUsers)))
	mux.Handle("POST /api/users", s.authMiddleware(http.HandlerFunc(s.handleCreateUser)))
	mux.Handle("DELETE /api/users/{name}", s.authMiddleware(http.HandlerFunc(s.handleDeleteUser)))

	mux.Handle("GET /api/users/{name}/keys", s.authMiddleware(http.HandlerFunc(s.handleListKeys)))
	mux.Handle("POST /api/users/{name}/keys", s.authMiddleware(http.HandlerFunc(s.handleCreateKey)))
	mux.Handle("DELETE /api/keys/{id}", s.authMiddleware(http.HandlerFunc(s.handleDeleteKey)))
	mux.Handle("PATCH /api/users/{name}/policies", s.authMiddleware(http.HandlerFunc(s.handleSetUserPolicies)))

	mux.Handle("GET /api/policies", s.authMiddleware(http.HandlerFunc(s.handleListPolicies)))
	mux.Handle("POST /api/policies", s.authMiddleware(http.HandlerFunc(s.handleCreatePolicy)))
	mux.Handle("PUT /api/policies/{name}", s.authMiddleware(http.HandlerFunc(s.handleUpdatePolicy)))
	mux.Handle("DELETE /api/policies/{name}", s.authMiddleware(http.HandlerFunc(s.handleDeletePolicy)))

	dist, err := fs.Sub(staticFiles, "web/dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(dist))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		f, err := dist.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err != nil {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})

	server := &http.Server{Addr: s.addr, Handler: mux}
	return server.ListenAndServe()
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("vexo_session")
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		sess, ok := s.sessions.Get(cookie.Value)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type sessionKey struct{}

func sessionFromContext(r *http.Request) *Session {
	if v, ok := r.Context().Value(sessionKey{}).(*Session); ok {
		return v
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "username and password are required")
		return
	}

	u, err := s.iam.GetUser(req.Username)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if u.Status != iam.StatusActive {
		writeJSONError(w, http.StatusForbidden, "user disabled")
		return
	}

	ok, err := s.iam.ValidatePassword(req.Username, req.Password)
	if err != nil || !ok {
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.sessions.Create(u.Username)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "vexo_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"username": u.Username})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("vexo_session")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"authenticated": false})
		return
	}
	sess, ok := s.sessions.Get(cookie.Value)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]interface{}{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"authenticated": true,
		"username":      sess.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("vexo_session")
	if err == nil {
		s.sessions.Delete(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "vexo_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	sess := sessionFromContext(r)
	if sess == nil {
		writeJSONError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": sess.Username})
}

func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	bkts, err := s.bucketStore.List()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, bkts)
}

func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	b, err := s.bucketStore.Create(req.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.bucketStore.Delete(name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	prefix := r.URL.Query().Get("prefix")
	objects, err := s.storage.List(name, prefix)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, objects)
}

func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key := r.PathValue("key")
	meta, err := s.storage.Put(name, key, r.Body, r.Header.Get("Content-Type"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key := r.PathValue("key")
	s.serveObjectBytes(w, r, name, key)
}

func (s *Server) serveObjectBytes(w http.ResponseWriter, r *http.Request, bucket, key string) {
	rc, meta, err := s.storage.Get(bucket, key)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	defer rc.Close()
	contentType := meta.ContentType
	if contentType == "" {
		if guessed := mime.TypeByExtension(filepath.Ext(key)); guessed != "" {
			contentType = guessed
		} else {
			contentType = "application/octet-stream"
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", formatInt64(meta.Size))
	io.Copy(w, rc)
}

func (s *Server) handlePublicGetObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	cfg, err := s.bucketStore.Get(bucket)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "bucket not found")
		return
	}

	if !cfg.Public {
		expStr := r.URL.Query().Get("exp")
		sig := r.URL.Query().Get("sig")
		if expStr == "" || sig == "" {
			writeJSONError(w, http.StatusForbidden, "this bucket is private")
			return
		}
		exp, err := strconv.ParseInt(expStr, 10, 64)
		if err != nil {
			writeJSONError(w, http.StatusForbidden, "invalid link")
			return
		}
		if time.Now().Unix() > exp {
			writeJSONError(w, http.StatusForbidden, "link expired")
			return
		}
		if !s.shareSigner.Verify(bucket, key, exp, sig) {
			writeJSONError(w, http.StatusForbidden, "invalid link")
			return
		}
	}

	s.serveObjectBytes(w, r, bucket, key)
}

func (s *Server) handleGetBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cfg, err := s.bucketStore.Get(name)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSetBucketVisibility(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Public bool `json:"public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfg, err := s.bucketStore.SetPublic(name, req.Public)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleShareObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Key              string `json:"key"`
		ExpiresInSeconds int64  `json:"expiresInSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" || req.ExpiresInSeconds <= 0 {
		writeJSONError(w, http.StatusBadRequest, "key and expiresInSeconds are required")
		return
	}
	if _, err := s.storage.Stat(name, req.Key); err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	expiresAt := time.Now().Unix() + req.ExpiresInSeconds
	sig := s.shareSigner.Sign(name, req.Key, expiresAt)

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/api/public/%s/%s?exp=%d&sig=%s", scheme, r.Host, name, req.Key, expiresAt, sig)

	writeJSON(w, http.StatusOK, map[string]any{
		"url":       url,
		"expiresAt": time.Unix(expiresAt, 0).UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRenameObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		OldKey string `json:"oldKey"`
		NewKey string `json:"newKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.OldKey == "" || req.NewKey == "" {
		writeJSONError(w, http.StatusBadRequest, "oldKey and newKey are required")
		return
	}
	meta, err := s.storage.Rename(name, req.OldKey, req.NewKey)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key := r.PathValue("key")
	if err := s.storage.Delete(name, key); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.iam.ListUsers()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, u := range users {
		u.PasswordHash = ""
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "password is required")
		return
	}
	u, err := s.iam.CreateUser(req.Username)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.iam.SetPassword(req.Username, req.Password); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.iam.DeleteUser(name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	keys, err := s.iam.ListAccessKeys(name)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type keyInfo struct {
		AccessKey string `json:"accessKey"`
		Username  string `json:"username"`
		Status    string `json:"status"`
		CreatedAt string `json:"createdAt"`
	}
	result := make([]keyInfo, len(keys))
	for i, k := range keys {
		result[i] = keyInfo{
			AccessKey: k.AccessKey,
			Username:  k.Username,
			Status:    k.Status,
			CreatedAt: k.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, secret, err := s.iam.CreateAccessKey(name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	keys, _ := s.iam.ListAccessKeys(name)
	akID := ""
	if len(keys) > 0 {
		akID = keys[len(keys)-1].AccessKey
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"accessKey": akID,
		"secret":    secret,
	})
}

func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.iam.DeleteAccessKey(id); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := s.policy.List()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

func (s *Server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string             `json:"name"`
		Statement []policy.Statement `json:"statement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || len(req.Statement) == 0 {
		writeJSONError(w, http.StatusBadRequest, "name and at least one statement are required")
		return
	}
	if _, err := s.policy.Get(req.Name); err == nil {
		writeJSONError(w, http.StatusBadRequest, "a policy with this name already exists")
		return
	}
	p := &policy.Policy{Version: "2012-10-17", Name: req.Name, Statement: req.Statement}
	if err := s.policy.Put(req.Name, p); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Statement []policy.Statement `json:"statement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Statement) == 0 {
		writeJSONError(w, http.StatusBadRequest, "at least one statement is required")
		return
	}
	if _, err := s.policy.Get(name); err != nil {
		writeJSONError(w, http.StatusNotFound, "policy not found")
		return
	}
	p := &policy.Policy{Version: "2012-10-17", Name: name, Statement: req.Statement}
	if err := s.policy.Put(name, p); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "root" {
		writeJSONError(w, http.StatusBadRequest, "the root policy cannot be deleted")
		return
	}
	if err := s.policy.Delete(name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleSetUserPolicies(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Policies []string `json:"policies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.iam.SetUserPolicies(name, req.Policies); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}