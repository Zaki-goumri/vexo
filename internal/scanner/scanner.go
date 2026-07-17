package scanner

import (
	"log"
	"sync"
	"time"

	"github.com/Zaki-goumri/vexo/internal/buckets"
	"github.com/Zaki-goumri/vexo/internal/lifecycle"
	"github.com/Zaki-goumri/vexo/internal/storage"
)

type Scanner struct {
	bucketStore *buckets.Store
	store       *storage.Store
	lcStore     *lifecycle.Store
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func New(bucketStore *buckets.Store, store *storage.Store, lcStore *lifecycle.Store) *Scanner {
	return &Scanner{
		bucketStore: bucketStore,
		store:       store,
		lcStore:     lcStore,
		interval:    60 * time.Second,
		stopCh:      make(chan struct{}),
	}
}

func (s *Scanner) WithInterval(d time.Duration) *Scanner {
	s.interval = d
	return s
}

func (s *Scanner) Start() {
	s.wg.Add(1)
	go s.loop()
}

func (s *Scanner) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t, d, err := s.ScanOnce()
			if err != nil {
				log.Printf("scanner: %v", err)
			}
			if t+d > 0 {
				log.Printf("scanner: transitioned=%d deleted=%d", t, d)
			}
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scanner) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scanner) ScanOnce() (transitioned, deleted int, err error) {
	bkts, err := s.bucketStore.List()
	if err != nil {
		return 0, 0, err
	}

	for _, bkt := range bkts {
		cfg, err := s.lcStore.GetLifecycle(bkt.Name)
		if err != nil {
			if err == lifecycle.ErrNoLifecycle {
				continue
			}
			log.Printf("scanner: get lifecycle for %s: %v", bkt.Name, err)
			continue
		}

		objects, err := s.store.List(bkt.Name, "")
		if err != nil {
			log.Printf("scanner: list objects for %s: %v", bkt.Name, err)
			continue
		}

		now := time.Now()
		for _, obj := range objects {
			action, tier := cfg.Evaluate(obj, now)
			switch action {
			case lifecycle.ActionTransition:
				if _, err := s.store.Transition(bkt.Name, obj.Key, tier); err != nil {
					log.Printf("scanner: transition %s/%s: %v", bkt.Name, obj.Key, err)
				} else {
					transitioned++
				}
			case lifecycle.ActionDelete:
				if err := s.store.Delete(bkt.Name, obj.Key); err != nil {
					log.Printf("scanner: delete %s/%s: %v", bkt.Name, obj.Key, err)
				} else {
					deleted++
				}
			}
		}
	}

	return transitioned, deleted, nil
}