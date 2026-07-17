package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Zaki-goumri/vexo/internal/db"
)

var (
	ErrPolicyNotFound      = errors.New("policy not found")
	ErrPolicyAlreadyExists = errors.New("policy already exists")
)

const EffectAllow = "Allow"
const EffectDeny = "Deny"

type Statement struct {
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource []string `json:"Resource"`
}

type Policy struct {
	Version   string      `json:"Version"`
	Name      string      `json:"Name"`
	Statement []Statement `json:"Statement"`
}

var RootPolicy = &Policy{
	Version: "2012-10-17",
	Name:    "root",
	Statement: []Statement{{
		Effect:   EffectAllow,
		Action:   []string{"*"},
		Resource: []string{"*"},
	}},
}

var ErrReadOnly = &Policy{
	Version: "2012-10-17",
	Name:    "readonly",
	Statement: []Statement{{
		Effect:   EffectAllow,
		Action:   []string{"s3:GetObject", "s3:ListBucket", "s3:GetBucketLocation"},
		Resource: []string{"*"},
	}},
}

type Store struct {
	meta     *db.DB
	cache    map[string]*regexp.Regexp
	cacheMu  sync.RWMutex
}

func NewStore(meta *db.DB) *Store {
	return &Store{
		meta:  meta,
		cache: make(map[string]*regexp.Regexp),
	}
}

func (s *Store) Put(name string, p *Policy) error {
	p.Name = name
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	return s.meta.Put("policies", name, data)
}

func (s *Store) Get(name string) (*Policy, error) {
	data, err := s.meta.Get("policies", name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}
	p.Name = name
	return &p, nil
}

func (s *Store) Delete(name string) error {
	return s.meta.Delete("policies", name)
}

func (s *Store) List() ([]*Policy, error) {
	var policies []*Policy
	err := s.meta.ForEach("policies", func(k, v []byte) error {
		var p Policy
		if err := json.Unmarshal(v, &p); err != nil {
			return fmt.Errorf("unmarshal policy %q: %w", k, err)
		}
		p.Name = string(k)
		policies = append(policies, &p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (s *Store) EvaluateByNames(policyNames []string, action, resource string) (bool, error) {
	policies := make([]*Policy, 0, len(policyNames))
	for _, name := range policyNames {
		p, err := s.Get(name)
		if err != nil {
			if errors.Is(err, ErrPolicyNotFound) {
				continue
			}
			return false, err
		}
		policies = append(policies, p)
	}
	return Evaluate(policies, action, resource), nil
}

func Evaluate(policies []*Policy, action, resource string) bool {
	allowed := false
	for _, p := range policies {
		for _, stmt := range p.Statement {
			if !matchAny(stmt.Action, action) {
				continue
			}
			if !matchAny(stmt.Resource, resource) {
				continue
			}
			switch stmt.Effect {
			case EffectDeny:
				return false
			case EffectAllow:
				allowed = true
			}
		}
	}
	return allowed
}

func matchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, value) {
			return true
		}
	}
	return false
}

func globToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '|', '{', '}', '^', '$', '\\', '[', ']':
			b.WriteByte('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteString("$")
	return b.String()
}

func (s *Store) compiledGlob(pattern string) (*regexp.Regexp, error) {
	s.cacheMu.RLock()
	if re, ok := s.cache[pattern]; ok {
		s.cacheMu.RUnlock()
		return re, nil
	}
	s.cacheMu.RUnlock()

	re, err := regexp.Compile(globToRegex(pattern))
	if err != nil {
		return nil, err
	}

	s.cacheMu.Lock()
	s.cache[pattern] = re
	s.cacheMu.Unlock()
	return re, nil
}

var sharedStore = NewStore(nil)

func matchPattern(pattern, value string) bool {
	if pattern == value {
		return true
	}
	if !strings.ContainsAny(pattern, "*?") {
		return false
	}
	re, err := sharedStore.compiledGlob(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(value)
}