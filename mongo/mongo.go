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

func NewMongoFactoryWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (factory *MongoFactory, err error) {
	if connTimeoutMs <= 0 {
		connTimeoutMs = DefConnectionTimeout
	}

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return
	}
	session.SetMode(mgo.PrimaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	factory = &MongoFactory{session: session}
	return
}

func NewMongoFactorySecondaryWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (factory *MongoFactory, err error) {

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return
	}
	session.SetMode(mgo.SecondaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	factory = &MongoFactory{session: session}
	return
}

func NewMongoFactoryDirectWithDsn(dsn string, connTimeoutMs, maxOpenConn int) (factory *MongoFactory, err error) {

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return
	}
	session.SetMode(mgo.Nearest, true)
	session.SetPoolLimit(maxOpenConn)
	session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	factory = &MongoFactory{session: session}
	return
}

func NewMongoFactoryWithParam(username, passwd, host string, port int,
	database string, connTimeoutMs, maxOpenConn int) (factory *MongoFactory, err error) {

	if connTimeoutMs <= 0 {
		connTimeoutMs = DefConnectionTimeout
	}

	dsn := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
		username, passwd, host, port, database)

	session, err := mgo.DialWithTimeout(dsn, time.Duration(connTimeoutMs)*time.Millisecond)
	if err != nil {
		return
	}

	session.SetMode(mgo.PrimaryPreferred, true)
	session.SetPoolLimit(maxOpenConn)
	factory.session.SetSyncTimeout(DefReadTimeout * time.Millisecond)

	factory = &MongoFactory{session: session}
	return
}

func (factory *MongoFactory) CreateIndex(dbName, collName string, index mgo.Index) error {
	session, err := factory.Get()
	if err != nil {
		return err
	}
	defer factory.Put(session)

	coll := session.DB(dbName).C(collName)
	coll.EnsureIndex(index)
	return nil
}

func (factory *MongoFactory) Get() (session *mgo.Session, err error) {
	session = factory.session.Copy()
	return
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
