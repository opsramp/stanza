package cachedpersister

import (
	"bytes"
	"encoding/json"
	"github.com/opsramp/stanza/operator/helper"
	"time"

	"github.com/opsramp/stanza/database"
	"go.etcd.io/bbolt"
)

type CachedBoltPersister struct {
	db               database.Database
	scope            string
	flushingInterval time.Duration
}

func NewPersister(db database.Database, scope string, flushingInterval helper.Duration) *CachedBoltPersister {
	return &CachedBoltPersister{
		db:               db,
		scope:            scope,
		flushingInterval: flushingInterval.Duration,
	}
}

type FileIdentifier struct {
	FingerPrint Fingerprint
	Offset      int64
}
type Fingerprint struct {
	// FirstBytes represents the first N bytes of a file
	FirstBytes []byte
}

func (b *CachedBoltPersister) Put(key string, value []byte) error {

	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(b.scope))
		if err != nil {
			return err
		}
		id := new(FileIdentifier)
		dec := json.NewDecoder(bytes.NewReader(value))
		if err := dec.Decode(&id); err != nil {
			return err
		}

		return bucket.Put([]byte(key), value)
	})

}

func (b *CachedBoltPersister) Get(key string) []byte {
	var value []byte
	b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(b.scope))
		if bucket == nil {
			return nil
		}
		value = bucket.Get([]byte(key))
		return nil
	})
	return value
}
