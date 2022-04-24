package cachedpersister

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/observiq/stanza/operator/builtin/input/filecheck"
	"github.com/observiq/stanza/operator/helper"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"
)

const test_scope = "test_scope"

type PersisterTestSuite struct {
	suite.Suite
	persister filecheck.Persister
	cancel    context.CancelFunc
}

func (p *PersisterTestSuite) SetupSuite() {
	dbFile, err := ioutil.TempFile("", "")
	p.Nil(err)
	_, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	options := &bbolt.Options{Timeout: 1 * time.Second}
	db, err := bbolt.Open(dbFile.Name(), 0600, options)
	p.Nil(err)
	p.persister = NewPersister(db, "test", helper.NewDuration(1*time.Second))

}

func (p *PersisterTestSuite) TearDownSuite() {
	p.cancel()
}

func TestPersisterSuite(t *testing.T) {
	suite.Run(t, new(PersisterTestSuite))
}

func (p *PersisterTestSuite) TestBoltPersisterGet() {
	value, ok := p.persister.Get("key1")
	p.False(ok)
	p.fillTestValues()
	p.persister.Flush()
	p.persister.ClearCache()

	value, ok = p.persister.Get("key1")
	p.False(ok)

	err := p.persister.LoadAll()
	p.Nil(err)

	value, ok = p.persister.Get("key1")
	p.True(ok)

	id := new(filecheck.FileIdentifier)
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.Decode(&id)
	p.Equal(id.FingerPrint.FirstBytes, []byte("test99"))
	p.Equal(id.Offset, int64(99))

	value, ok = p.persister.Get("key2")
	p.True(ok)

	dec = json.NewDecoder(bytes.NewReader(value))
	dec.Decode(&id)
	p.Equal(id.FingerPrint.FirstBytes, []byte("test333"))
	p.Equal(id.Offset, int64(380))

}

func (p *PersisterTestSuite) TestTicker() {

	p.SetupSuite()
	p.fillTestValues()
	//wait for flushing
	<-time.After(2 * time.Second)
	p.persister.ClearCache()
	p.persister.LoadAll()
	_, ok := p.persister.Get("key1")
	p.True(ok)

}

func (p *PersisterTestSuite) fillTestValues() {
	//putting test values
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)

	key1 := filecheck.FileIdentifier{
		FingerPrint: &filecheck.Fingerprint{FirstBytes: []byte("test99")},
		Offset:      99,
	}

	err := enc.Encode(key1)
	p.Nil(err)
	p.persister.Put("key1", buf.Bytes())

	key2 := filecheck.FileIdentifier{
		FingerPrint: &filecheck.Fingerprint{FirstBytes: []byte("test333")},
		Offset:      380,
	}

	buf = new(bytes.Buffer)
	enc = json.NewEncoder(buf)
	err = enc.Encode(key2)
	p.Nil(err)
	p.persister.Put("key2", buf.Bytes())

	p.Nil(err)
}
