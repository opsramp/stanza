package filecheck

import "context"

// Persister is a helper used to persist data
type Persister interface {
	StartFlusher(ctx context.Context)
	Get(key string) ([]byte, bool)
	Put(key string, value []byte)
	LoadAll() error
	Flush() error
	Clear() error
	IsEmpty() bool
}
