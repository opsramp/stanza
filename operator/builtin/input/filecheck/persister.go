package filecheck

// Persister is a helper used to persist data
type Persister interface {
	Get(key string) []byte
	Put(key string, value []byte) error
	Close() error
}
