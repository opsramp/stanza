package filecheck

import (
	"github.com/observiq/stanza/database"
	"go.etcd.io/bbolt"
)

const bucket = "checkpoints"

// Persister is a helper used to persist data
type Persister interface {
	GetAll() map[string][]byte
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
}

type BoltPersister struct {
	scope []byte
	db    database.Database
}

func NewPersister(db database.Database, scope string) *BoltPersister {
	return &BoltPersister{
		scope: []byte(scope),
		db:    db,
	}
}
func (b *BoltPersister) GetAll() map[string][]byte {
	result := make(map[string][]byte)

	b.db.View(func(tx *bbolt.Tx) error {

		bucket := tx.Bucket([]byte(bucket))

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			result[string(k)] = v
		}

		return nil
	})
	return result
}

func (b *BoltPersister) Get(key string) ([]byte, error) {
	var value []byte

	err := b.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucket))
		value = bucket.Get([]byte(key))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (b *BoltPersister) Put(key string, value []byte) error {

	err := b.db.Update(func(tx *bbolt.Tx) error {
		bucket, dbErr := tx.CreateBucketIfNotExists([]byte(bucket))
		if dbErr != nil {
			return dbErr
		}
		dbErr = bucket.Put([]byte(key), value)
		return dbErr
	})

	return err
}
