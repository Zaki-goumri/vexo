package api

import (
	"context"
	"net/http"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

type Server struct {
	bucketStore *buckets.Store
	storage     *storage.Store
	addr        string
	router      *http.ServeMux
	server      *http.Server
}

func NewServer(bucketStore *buckets.Store, store *storage.Store, addr string) *Server {
	s := &Server{
		bucketStore: bucketStore,
		storage:     store,
		addr:        addr,
		router:      http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.router.HandleFunc("GET /", s.listBuckets)

	s.router.HandleFunc("PUT /{bucket}", s.createBucket)
	s.router.HandleFunc("DELETE /{bucket}", s.deleteBucket)
	s.router.HandleFunc("HEAD /{bucket}", s.headBucket)
	s.router.HandleFunc("GET /{bucket}", s.listObjects)

	s.router.HandleFunc("PUT /{bucket}/{key...}", s.putObject)
	s.router.HandleFunc("GET /{bucket}/{key...}", s.getObject)
	s.router.HandleFunc("HEAD /{bucket}/{key...}", s.headObject)
	s.router.HandleFunc("DELETE /{bucket}/{key...}", s.deleteObject)
}

func (s *Server) ListenAndServe() error {
	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.router,
	}
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.router
}