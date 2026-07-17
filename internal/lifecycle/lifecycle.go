package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

var (
	ErrNoLifecycle = errors.New("no lifecycle configured")
)

const (
	StatusEnabled  = "Enabled"
	StatusDisabled = "Disabled"
)

type Action int

const (
	ActionNone Action = iota
	ActionTransition
	ActionDelete
)

func (a Action) String() string {
	switch a {
	case ActionTransition:
		return "Transition"
	case ActionDelete:
		return "Delete"
	default:
		return "None"
	}
}

type Transition struct {
	Days int    `json:"days"`
	Tier string `json:"tier"`
}

type Expiration struct {
	Days int `json:"days"`
}

type Rule struct {
	ID          string       `json:"id"`
	Prefix      string       `json:"prefix"`
	Status      string       `json:"status"`
	Transitions []Transition `json:"transitions"`
	Expiration  *Expiration  `json:"expiration"`
}

type Config struct {
	Rules []Rule `json:"rules"`
}

type Store struct {
	bucketStore *buckets.Store
}

func NewStore(bucketStore *buckets.Store) *Store {
	return &Store{bucketStore: bucketStore}
}

func (s *Store) SetLifecycle(bucketName string, cfg *Config) error {
	bc, err := s.bucketStore.Get(bucketName)
	if err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal lifecycle: %w", err)
	}
	bc.Lifecycle = data
	bcJSON, err := json.Marshal(bc)
	if err != nil {
		return fmt.Errorf("marshal bucket config: %w", err)
	}
	return s.bucketStore.Meta().Put("buckets", bucketName, bcJSON)
}

func (s *Store) GetLifecycle(bucketName string) (*Config, error) {
	bc, err := s.bucketStore.Get(bucketName)
	if err != nil {
		return nil, err
	}
	if bc.Lifecycle == nil || len(bc.Lifecycle) == 0 {
		return nil, ErrNoLifecycle
	}
	var cfg Config
	if err := json.Unmarshal(bc.Lifecycle, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal lifecycle: %w", err)
	}
	return &cfg, nil
}

func (s *Store) DeleteLifecycle(bucketName string) error {
	bc, err := s.bucketStore.Get(bucketName)
	if err != nil {
		return err
	}
	bc.Lifecycle = nil
	bcJSON, err := json.Marshal(bc)
	if err != nil {
		return err
	}
	return s.bucketStore.Meta().Put("buckets", bucketName, bcJSON)
}

func (c *Config) Evaluate(meta *storage.ObjectMeta, now time.Time) (Action, string) {
	if meta == nil {
		return ActionNone, ""
	}

	delete := false
	transitionTier := ""

	for _, rule := range c.Rules {
		if rule.Status != StatusEnabled {
			continue
		}
		if rule.Prefix != "" && !strings.HasPrefix(meta.Key, rule.Prefix) {
			continue
		}

		daysSinceCreation := now.Sub(meta.CreatedAt).Hours() / 24
		daysSinceAccess := now.Sub(meta.LastAccessedAt).Hours() / 24

		if rule.Expiration != nil {
			if int(daysSinceCreation) >= rule.Expiration.Days {
				delete = true
			}
		}

		applicable := make([]Transition, 0)
		for _, tr := range rule.Transitions {
			if int(daysSinceAccess) >= tr.Days && meta.Tier != tr.Tier {
				if isColderOrEqual(tr.Tier, meta.Tier) {
					continue
				}
				applicable = append(applicable, tr)
			}
		}

		if len(applicable) > 0 {
			sort.Slice(applicable, func(i, j int) bool {
				return tierRank(applicable[i].Tier) > tierRank(applicable[j].Tier)
			})
			transitionTier = applicable[0].Tier
		}
	}

	if delete {
		return ActionDelete, ""
	}
	if transitionTier != "" {
		return ActionTransition, transitionTier
	}
	return ActionNone, ""
}

func tierRank(tier string) int {
	switch tier {
	case storage.TierCold:
		return 3
	case storage.TierInfrequent:
		return 2
	case storage.TierHot:
		return 1
	default:
		return 0
	}
}

func isColderOrEqual(candidate, current string) bool {
	return tierRank(candidate) <= tierRank(current)
}