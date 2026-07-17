package iam

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Zaki-goumri/vexo/internal/db"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrGroupNotFound      = errors.New("group not found")
	ErrGroupAlreadyExists = errors.New("group already exists")
	ErrAccessKeyNotFound  = errors.New("access key not found")
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"

	accessKeyLen = 20
	secretLen    = 40
)

type User struct {
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Groups   []string  `json:"groups"`
	Policies []string  `json:"policies"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AccessKey struct {
	AccessKey   string    `json:"accessKey"`
	PlainSecret string    `json:"plainSecret"`
	Username    string    `json:"username"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Group struct {
	Name      string    `json:"name"`
	Members   []string  `json:"members"`
	Policies  []string  `json:"policies"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	meta *db.DB
}

func NewStore(meta *db.DB) *Store {
	return &Store{meta: meta}
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:n], nil
}

func (s *Store) CreateUser(username string) (*User, error) {
	exists, err := s.meta.Has("users", username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}
	now := time.Now()
	u := &User{
		Username:  username,
		Status:    StatusActive,
		Groups:   []string{},
		Policies: []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	data, err := json.Marshal(u)
	if err != nil {
		return nil, err
	}
	if err := s.meta.Put("users", username, data); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUser(username string) (*User, error) {
	data, err := s.meta.Get("users", username)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	var u User
	if err := json.Unmarshal(data, &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

func (s *Store) ListUsers() ([]*User, error) {
	var users []*User
	err := s.meta.ForEach("users", func(_, v []byte) error {
		var u User
		if err := json.Unmarshal(v, &u); err != nil {
			return fmt.Errorf("unmarshal user: %w", err)
		}
		users = append(users, &u)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *Store) DeleteUser(username string) error {
	if _, err := s.GetUser(username); err != nil {
		return err
	}
	keys, err := s.ListAccessKeys(username)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := s.meta.Delete("accesskeys", k.AccessKey); err != nil {
			return err
		}
	}
	return s.meta.Delete("users", username)
}

func (s *Store) SetUserStatus(username, status string) error {
	u, err := s.GetUser(username)
	if err != nil {
		return err
	}
	u.Status = status
	u.UpdatedAt = time.Now()
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return s.meta.Put("users", username, data)
}

func (s *Store) CreateAccessKey(username string) (*AccessKey, string, error) {
	if _, err := s.GetUser(username); err != nil {
		return nil, "", err
	}
	accessKeyID, err := randomString(accessKeyLen)
	if err != nil {
		return nil, "", err
	}
	plaintextSecret, err := randomString(secretLen)
	if err != nil {
		return nil, "", err
	}
	ak := &AccessKey{
		AccessKey:   accessKeyID,
		PlainSecret: plaintextSecret,
		Username:    username,
		Status:      StatusActive,
		CreatedAt:   time.Now(),
	}
	data, err := json.Marshal(ak)
	if err != nil {
		return nil, "", err
	}
	if err := s.meta.Put("accesskeys", accessKeyID, data); err != nil {
		return nil, "", err
	}
	return ak, plaintextSecret, nil
}

func (s *Store) GetAccessKey(accessKeyID string) (*AccessKey, error) {
	data, err := s.meta.Get("accesskeys", accessKeyID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrAccessKeyNotFound
		}
		return nil, err
	}
	var ak AccessKey
	if err := json.Unmarshal(data, &ak); err != nil {
		return nil, fmt.Errorf("unmarshal access key: %w", err)
	}
	return &ak, nil
}

func (s *Store) ListAccessKeys(username string) ([]*AccessKey, error) {
	var keys []*AccessKey
	err := s.meta.ForEach("accesskeys", func(_, v []byte) error {
		var ak AccessKey
		if err := json.Unmarshal(v, &ak); err != nil {
			return fmt.Errorf("unmarshal access key: %w", err)
		}
		if ak.Username == username {
			keys = append(keys, &ak)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *Store) DeleteAccessKey(accessKeyID string) error {
	if _, err := s.GetAccessKey(accessKeyID); err != nil {
		return err
	}
	return s.meta.Delete("accesskeys", accessKeyID)
}

func (s *Store) ValidateSecret(accessKeyID, plaintextSecret string) (bool, error) {
	ak, err := s.GetAccessKey(accessKeyID)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(ak.PlainSecret), []byte(plaintextSecret)) == 1, nil
}

func (s *Store) CreateGroup(name string) (*Group, error) {
	exists, err := s.meta.Has("groups", name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrGroupAlreadyExists
	}
	g := &Group{
		Name:      name,
		Members:   []string{},
		Policies:  []string{},
		CreatedAt: time.Now(),
	}
	data, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	if err := s.meta.Put("groups", name, data); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) GetGroup(name string) (*Group, error) {
	data, err := s.meta.Get("groups", name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrGroupNotFound
		}
		return nil, err
	}
	var g Group
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("unmarshal group: %w", err)
	}
	return &g, nil
}

func (s *Store) ListGroups() ([]*Group, error) {
	var groups []*Group
	err := s.meta.ForEach("groups", func(_, v []byte) error {
		var g Group
		if err := json.Unmarshal(v, &g); err != nil {
			return fmt.Errorf("unmarshal group: %w", err)
		}
		groups = append(groups, &g)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *Store) DeleteGroup(name string) error {
	if _, err := s.GetGroup(name); err != nil {
		return err
	}
	return s.meta.Delete("groups", name)
}

func (s *Store) AddUserToGroup(groupName, username string) error {
	g, err := s.GetGroup(groupName)
	if err != nil {
		return err
	}
	for _, m := range g.Members {
		if m == username {
			return nil
		}
	}
	g.Members = append(g.Members, username)
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return s.meta.Put("groups", groupName, data)
}

func (s *Store) RemoveUserFromGroup(groupName, username string) error {
	g, err := s.GetGroup(groupName)
	if err != nil {
		return err
	}
	for i, m := range g.Members {
		if m == username {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			break
		}
	}
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	return s.meta.Put("groups", groupName, data)
}

func (s *Store) BootstrapRoot(rootDir string) error {
	users, err := s.ListUsers()
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return nil
	}

	username := os.Getenv("VEXO_ROOT_USER")
	if username == "" {
		username = "zaki"
	}

	u, err := s.CreateUser(username)
	if err != nil {
		return err
	}
	u.Policies = []string{"root"}
	now := time.Now()
	u.UpdatedAt = now
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	if err := s.meta.Put("users", username, data); err != nil {
		return err
	}

	_, plaintextSecret, err := s.CreateAccessKey(username)
	if err != nil {
		return err
	}

	accessKeys, err := s.ListAccessKeys(username)
	if err != nil {
		return err
	}
	if len(accessKeys) == 0 {
		return fmt.Errorf("no access key created")
	}
	akID := accessKeys[0].AccessKey

	keyFile := filepath.Join(rootDir, ".vexo.root.key")
	creds := fmt.Sprintf("%s:%s\n", akID, plaintextSecret)
	if err := os.WriteFile(keyFile, []byte(creds), 0o600); err != nil {
		return fmt.Errorf("write root key file: %w", err)
	}

	fmt.Printf("=== Vexo root credentials ===\n")
	fmt.Printf("Access Key: %s\n", akID)
	fmt.Printf("Secret:     %s\n", plaintextSecret)
	fmt.Printf("Saved to:   %s\n", keyFile)
	fmt.Printf("==============================\n")

	return nil
}