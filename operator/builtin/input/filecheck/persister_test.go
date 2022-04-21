package filecheck

import (
	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"
	"io/ioutil"
	"testing"
	"time"
)

const test_scope = "test_scope"

type PersisterTestSuite struct {
	suite.Suite
	p Persister
}

func (p *PersisterTestSuite) SetupSuite() {
	dbFile, err := ioutil.TempFile("", "")
	p.Nil(err)

	options := &bbolt.Options{Timeout: 1 * time.Second}
	db, err := bbolt.Open(dbFile.Name(), 0600, options)
	p.Nil(err)
	p.p = NewPersister(db, test_scope)
}

func TestPersisterSuite(t *testing.T) {
	suite.Run(t, new(PersisterTestSuite))
}

func (p *PersisterTestSuite) TestBoltPersisterBucket() {
	err := p.p.Put("test", []byte("value"))
	p.Nil(err)
	value, err := p.p.Get("test")
	p.Equal(value, []byte("value"))
	err = p.p.Put("test1", []byte("value1"))
	p.Nil(err)
	values := p.p.GetAll()
	p.Len(values, 2)
	p.NotNil(values["test"])
	p.NotNil(values["test1"])
}
