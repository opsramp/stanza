package boltdbpersister

import (
	"bytes"
	"encoding/json"
	"github.com/opsramp/stanza/operator/helper"
	"time"

	"github.com/opsramp/stanza/database"
	"go.etcd.io/bbolt"
)

// BoltPersister BoltDB implementation
type BoltPersister struct {
	db               database.Database
	scope            string
	flushingInterval time.Duration
}

// NewPersister -
func NewPersister(db database.Database, scope string, flushingInterval helper.Duration) *BoltPersister {
	return &BoltPersister{
		db:               db,
		scope:            scope,
		flushingInterval: flushingInterval.Duration,
	}
}

// FileIdentifier -
type FileIdentifier struct {
	FingerPrint Fingerprint
	Offset      int64
}

// Fingerprint -
type Fingerprint struct {
	// FirstBytes represents the first N bytes of a file
	FirstBytes []byte
}

// Put value to storage
func (b *BoltPersister) Put(key string, value []byte) error {

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

// Get By key
func (b *BoltPersister) Get(key string) []byte {
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
