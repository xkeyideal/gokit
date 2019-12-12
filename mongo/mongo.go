package mongo

import (
	"fmt"
	"time"

	"github.com/globalsign/mgo"
)

const (
	DefConnectionTimeout = 3000
	DefReadTimeout       = 3000
)

type MongoFactory struct {
	session *mgo.Session
}

func NewMongoFactoryWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (*MongoFactory, error) {
	if connTimeoutMs <= 0 {
		connTimeoutMs = DefConnectionTimeout
	}

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	session.SetMode(mgo.PrimaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	return &MongoFactory{session: session}, nil
}

func NewMongoFactorySecondaryWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (*MongoFactory, error) {
	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	session.SetMode(mgo.SecondaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	return &MongoFactory{session: session}, nil
}

func NewMongoFactoryDirectWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (*MongoFactory, error) {
	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	session.SetMode(mgo.Nearest, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	return &MongoFactory{session: session}, nil
}

func NewMongoFactoryWithParam(username, passwd, host string, port int,
	database string, connTimeoutMs, maxOpenConn int) (*MongoFactory, error) {

	if connTimeoutMs <= 0 {
		connTimeoutMs = DefConnectionTimeout
	}

	dsn := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
		username, passwd, host, port, database)

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}

	session.SetMode(mgo.PrimaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	return &MongoFactory{session: session}, nil
}

func (factory *MongoFactory) CreateIndex(dbName, collName string, index mgo.Index) error {
	session := factory.Get()
	defer factory.Put(session)

	coll := session.DB(dbName).C(collName)

	return coll.EnsureIndex(index)
}

func (factory *MongoFactory) Get() *mgo.Session {
	return factory.session.Copy()
}

func (factory *MongoFactory) Put(session *mgo.Session) {
	if session != nil {
		session.Close()
	}
}

func (factory *MongoFactory) Close() {
	if factory.session != nil {
		factory.session.Close()
	}
}
