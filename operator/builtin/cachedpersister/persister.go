package cachedpersister

import (
	"context"
	"fmt"
	"github.com/observiq/stanza/operator/helper"
	"sync"
	"time"

	"github.com/observiq/stanza/database"
	"go.etcd.io/bbolt"
)

type CachedBoltPersister struct {
	db               database.Database
	cache            map[string][]byte
	mux              sync.Mutex
	scope            string
	flushingInterval time.Duration
}

func NewPersister(db database.Database, scope string, flushingInterval helper.Duration) *CachedBoltPersister {
	return &CachedBoltPersister{
		db:               db,
		cache:            make(map[string][]byte),
		scope:            scope,
		flushingInterval: flushingInterval.Duration,
	}

}

func (b *CachedBoltPersister) StartFlusher(ctx context.Context) {
	ticker := time.NewTicker(b.flushingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.Flush()
			b.db.Close()
			return
		case <-ticker.C:
			b.Flush()
		}
	}

}

func (b *CachedBoltPersister) LoadAll() error {
	return b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(b.scope))

		if bucket == nil {
			return fmt.Errorf("bucket missing for scope %s", b.scope)
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			b.cache[string(k)] = v
		}

		return nil
	})
}

func (b *CachedBoltPersister) Flush() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(b.scope))
		if err != nil {
			return err
		}

		b.mux.Lock()
		for k, v := range b.cache {
			err := bucket.Put([]byte(k), v)
			if err != nil {
				return err
			}
		}
		b.mux.Unlock()
		return nil
	})
}

func (b *CachedBoltPersister) Clear() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(b.scope))
		err := bucket.DeleteBucket([]byte(b.scope))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucket([]byte(b.scope))
		return err
	})

}

func (b *CachedBoltPersister) ClearCache() {
	b.cache = make(map[string][]byte)
}

func (b *CachedBoltPersister) Put(key string, value []byte) {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.cache[key] = value
}

func (b *CachedBoltPersister) Get(key string) ([]byte, bool) {
	b.mux.Lock()
	defer b.mux.Unlock()
	id, ok := b.cache[key]

	return id, ok
}
