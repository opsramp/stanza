package leveldbpersister

import (
	"time"

	"github.com/opsramp/stanza/operator/helper"
	"github.com/syndtr/goleveldb/leveldb"
)

// LevelDBPersister - LST tree implementation
type LevelDBPersister struct {
	db               *leveldb.DB
	scope            string
	flushingInterval time.Duration
}

// NewLevelDBPersister constructor
func NewLevelDBPersister(scope string, flushingInterval helper.Duration, databaseFile string) (*LevelDBPersister, error) {
	db, err := leveldb.OpenFile(databaseFile, nil)
	if err != nil {
		return nil, err
	}

	return &LevelDBPersister{
		db:               db,
		scope:            scope,
		flushingInterval: flushingInterval.Duration,
	}, nil
}

// Put -
func (b *LevelDBPersister) Put(key string, value []byte) error {
	return b.db.Put([]byte(key), value, nil)
}

// Get by key
func (b *LevelDBPersister) Get(key string) []byte {
	value, err := b.db.Get([]byte(key), nil)
	if err != nil {
		return nil
	}
	return value
}

func (b *LevelDBPersister) Close() error {
	return b.db.Close()
}

// StubDBPersister empty database implementaton
type StubDBPersister struct{}

// NewStubDBPersister -
func NewStubDBPersister() *StubDBPersister {
	return &StubDBPersister{}
}

// Put -
func (b *StubDBPersister) Put(key string, value []byte) error {
	return nil
}

// Get -
func (b *StubDBPersister) Get(key string) []byte {
	return nil
}

func (b *StubDBPersister) Close() error {
	return nil
}
