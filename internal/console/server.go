package console

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

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
	addr        string
}

func NewServer(iamStore *iam.Store, policyStore *policy.Store, bucketStore *buckets.Store, store *storage.Store, addr string) *Server {
	return &Server{
		sessions:    NewSessionStore(),
		iam:         iamStore,
		policy:      policyStore,
		bucketStore: bucketStore,
		storage:     store,
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
	mux.Handle("DELETE /api/buckets/{name}", s.authMiddleware(http.HandlerFunc(s.handleDeleteBucket)))

	mux.Handle("GET /api/buckets/{name}/objects", s.authMiddleware(http.HandlerFunc(s.handleListObjects)))
	mux.Handle("PUT /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handlePutObject)))
	mux.Handle("GET /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handleGetObject)))
	mux.Handle("DELETE /api/buckets/{name}/objects/{key...}", s.authMiddleware(http.HandlerFunc(s.handleDeleteObject)))

	mux.Handle("GET /api/users", s.authMiddleware(http.HandlerFunc(s.handleListUsers)))
	mux.Handle("POST /api/users", s.authMiddleware(http.HandlerFunc(s.handleCreateUser)))
	mux.Handle("DELETE /api/users/{name}", s.authMiddleware(http.HandlerFunc(s.handleDeleteUser)))

	mux.Handle("GET /api/users/{name}/keys", s.authMiddleware(http.HandlerFunc(s.handleListKeys)))
	mux.Handle("POST /api/users/{name}/keys", s.authMiddleware(http.HandlerFunc(s.handleCreateKey)))
	mux.Handle("DELETE /api/keys/{id}", s.authMiddleware(http.HandlerFunc(s.handleDeleteKey)))

	mux.Handle("GET /api/policies", s.authMiddleware(http.HandlerFunc(s.handleListPolicies)))

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
		AccessKey string `json:"accessKey"`
		Secret    string `json:"secret"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AccessKey == "" || req.Secret == "" {
		writeJSONError(w, http.StatusBadRequest, "accessKey and secret are required")
		return
	}

	ak, err := s.iam.GetAccessKey(req.AccessKey)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if ak.Status != iam.StatusActive {
		writeJSONError(w, http.StatusForbidden, "access key disabled")
		return
	}

	ok, err := s.iam.ValidateSecret(req.AccessKey, req.Secret)
	if err != nil || !ok {
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if ak.Status != iam.StatusActive {
		writeJSONError(w, http.StatusForbidden, "access key disabled")
		return
	}

	token, err := s.sessions.Create(ak.Username, req.AccessKey)
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
	writeJSON(w, http.StatusOK, map[string]string{"username": ak.Username})
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
	meta, err := s.storage.Put(name, key, r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key := r.PathValue("key")
	rc, meta, err := s.storage.Get(name, key)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", formatInt64(meta.Size))
	io.Copy(w, rc)
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
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	u, err := s.iam.CreateUser(req.Username)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
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

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}