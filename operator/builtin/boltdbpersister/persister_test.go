package boltdbpersister

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/opsramp/stanza/operator/builtin/input/filecheck"
	"github.com/opsramp/stanza/operator/helper"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"
)

const testScope = "testScope"

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
	value := p.persister.Get("key1")
	p.Nil(value)
	p.fillTestValues()

	value = p.persister.Get("key1")
	p.NotNil(value)

	id := new(filecheck.FileIdentifier)
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.Decode(&id)
	p.Equal(id.FingerPrint.FirstBytes, []byte("test99"))
	p.Equal(id.Offset, int64(99))

	value = p.persister.Get("key2")
	p.NotNil(value)

	dec = json.NewDecoder(bytes.NewReader(value))
	dec.Decode(&id)
	p.Equal(id.FingerPrint.FirstBytes, []byte("test333"))
	p.Equal(id.Offset, int64(380))

}

func (p *PersisterTestSuite) TestTicker() {

	p.SetupSuite()
	p.fillTestValues()
	//wait for flushing
	<-time.After(1 * time.Second)

	value := p.persister.Get("key1")
	p.NotNil(value)

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
