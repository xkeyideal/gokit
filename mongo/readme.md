
# tclog
    import "github.com/xkeyideal/gokit/mongo"

1. 支持dsn和参数两种方式
2. 默认的mongo连接模式采用PrimaryPreferred
3. mongo的golang驱动采用的是[mgo](https://github.com/globalsign/mgo),原来的[go-mgo/mgo](https://github.com/go-mgo/mgo)已经停止维护

### func NewMongoFactoryWithDsn

``` go
func NewMongoFactoryWithDsn(dsn string, connTimeoutMS, maxOpenConn int) (factory *MongoFactory, err error)
```

### func NewMongoFactoryDirectWithDsn

``` go
func NewMongoFactoryDirectWithDsn(dsn string, connTimeoutMS, maxOpenConn int) (factory *MongoFactory, err error)
```

此方式的mongo模式采用的是Nearest

### func NewMongoFactoryWithParam

``` go
func NewMongoFactoryWithParam(username, passwd, host string, port int,
	database string, connTimeoutMs, maxOpenConn int) (factory *MongoFactory, err error)
```

### func CreateIndex

``` go
func (factory *MongoFactory) CreateIndex(dbName, collName string, index mgo.Index) error
```

用于创建索引
	
## Example

```go

	var GlobalMongo *mongo.MongoFactory

	GlobalMongo, err = mongo.NewMongoFactoryWithDsn(mongoConnURL, maxOpenConn)
	if err != nil {
		fmt.Println(err)
	}
	
	session, err := global.GlobalMongo.Get()

	if err != nil {
		return err
	}

	defer global.GlobalMongo.Put(session)

	coll := session.DB(database).C(collection)

	err = coll.Find(query).One(&result)
	
```
