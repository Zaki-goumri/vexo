package api

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

type Owner struct {
	ID          string `xml:"ID"`
	DisplayName string `xml:"DisplayName"`
}

type BucketXML struct {
	Name         string    `xml:"Name"`
	CreationDate time.Time `xml:"CreationDate"`
}

type ListAllMyBucketsResult struct {
	XMLName xml.Name   `xml:"ListAllMyBucketsResult"`
	Owner   Owner       `xml:"Owner"`
	Buckets []BucketXML `xml:"Buckets>Bucket"`
}

type ObjectXML struct {
	Key          string    `xml:"Key"`
	Size         int64     `xml:"Size"`
	ETag         string    `xml:"ETag"`
	LastModified time.Time `xml:"LastModified"`
	StorageClass string    `xml:"StorageClass"`
}

type ListBucketResult struct {
	XMLName        xml.Name     `xml:"ListBucketResult"`
	Name           string       `xml:"Name"`
	Prefix         string       `xml:"Prefix"`
	KeyCount       int          `xml:"KeyCount"`
	MaxKeys        int          `xml:"MaxKeys"`
	IsTruncated    bool         `xml:"IsTruncated"`
	Contents       []ObjectXML  `xml:"Contents"`
}

type ErrorResponse struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource"`
	RequestID string   `xml:"RequestId"`
}

func writeXML(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	resp := ErrorResponse{
		Code:      code,
		Message:   message,
		Resource:  r.URL.Path,
		RequestID: r.Header.Get("x-amz-request-id"),
	}
	writeXML(w, status, resp)
}

func (s *Server) listBuckets(w http.ResponseWriter, r *http.Request) {
	bkts, err := s.bucketStore.List()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := ListAllMyBucketsResult{
		Owner: Owner{ID: "vexo", DisplayName: "vexo"},
		Buckets: make([]BucketXML, 0, len(bkts)),
	}
	for _, b := range bkts {
		result.Buckets = append(result.Buckets, BucketXML{
			Name:         b.Name,
			CreationDate: b.CreatedAt,
		})
	}
	writeXML(w, http.StatusOK, result)
}

func (s *Server) createBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("bucket")

	if _, err := s.bucketStore.Create(name); err != nil {
		switch {
		case errors.Is(err, buckets.ErrBucketAlreadyExists):
			writeError(w, r, http.StatusConflict, "BucketAlreadyExists", err.Error())
		case errors.Is(err, buckets.ErrInvalidBucketName):
			writeError(w, r, http.StatusBadRequest, "InvalidBucketName", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}

	w.Header().Set("Location", "/"+name)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("bucket")

	if err := s.bucketStore.Delete(name); err != nil {
		switch {
		case errors.Is(err, buckets.ErrBucketNotFound):
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", err.Error())
		case errors.Is(err, buckets.ErrBucketNotEmpty):
			writeError(w, r, http.StatusConflict, "BucketNotEmpty", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) headBucket(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("bucket")

	if _, err := s.bucketStore.Get(name); err != nil {
		if errors.Is(err, buckets.ErrBucketNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) listObjects(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")

	if _, err := s.bucketStore.Get(name); err != nil {
		if errors.Is(err, buckets.ErrBucketNotFound) {
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	objects, err := s.storage.List(name, prefix)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}

	result := ListBucketResult{
		Name:     name,
		Prefix:   prefix,
		KeyCount: len(objects),
		MaxKeys:  1000,
		Contents: make([]ObjectXML, 0, len(objects)),
	}
	for _, obj := range objects {
		result.Contents = append(result.Contents, ObjectXML{
			Key:          obj.Key,
			Size:         obj.Size,
			ETag:         obj.ETag,
			LastModified: obj.CreatedAt,
			StorageClass: obj.Tier,
		})
	}
	writeXML(w, http.StatusOK, result)
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := r.PathValue("key")

	meta, err := s.storage.Put(bucket, key, r.Body)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrBucketNotFound):
			writeError(w, r, http.StatusNotFound, "NoSuchBucket", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}

	w.Header().Set("ETag", `"`+meta.ETag+`"`)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := strings.TrimPrefix(r.PathValue("key"), "/")

	rc, meta, err := s.storage.Get(bucket, key)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrObjectNotFound):
			writeError(w, r, http.StatusNotFound, "NoSuchKey", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", formatInt64(meta.Size))
	w.Header().Set("ETag", `"`+meta.ETag+`"`)
	w.Header().Set("Last-Modified", meta.CreatedAt.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)

	if r.Method != http.MethodHead {
		io.Copy(w, rc)
	}
}

func (s *Server) headObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := strings.TrimPrefix(r.PathValue("key"), "/")

	meta, err := s.storage.Stat(bucket, key)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrObjectNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", formatInt64(meta.Size))
	w.Header().Set("ETag", `"`+meta.ETag+`"`)
	w.Header().Set("Last-Modified", meta.CreatedAt.UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	key := strings.TrimPrefix(r.PathValue("key"), "/")

	if err := s.storage.Delete(bucket, key); err != nil {
		switch {
		case errors.Is(err, storage.ErrObjectNotFound):
			writeError(w, r, http.StatusNotFound, "NoSuchKey", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}