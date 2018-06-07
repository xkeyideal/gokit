# nsqdlb
    import "github.com/xkeyideal/gokit/nsqdlb"

1. 多个nsqd的负载均衡（随机算法）
2. 自动摘除连不通的nsqd，并通知业务
3. 间隔60秒自动Ping一次
4. 日志统一采用Info级别

## Example

```go
func cb(addr string, err error) {
	if err != nil {
		fmt.Println(addr, "Err: ", err)
	}
}

func main() {
	nsqdProd, err := nsqdlb.NewNSQProducer([]string{"192.168.1.1:4161", "192.168.1.2:4161"}, nsq.NewConfig(), cb, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	
	for i := 0; i < 10; i ++ {
		err := nsqdProd.Publish("topic",[]byte("hello world"))
		if err != nil {
			fmt.Println(err)
		}
	}
	
	nsqdProd.Close()
}
```

详细使用方法和功能测试、并发测试用例请参考(https://github.com/xkeyideal/gokit/blob/master/nsqdlb/main/main.go)

### func ErrCb

```go
type ErrCb func(addr string, err error)
```

### func NewNSQProducer

```go
func NewNSQProducer(nsqdAddrs []string, cfg *nsq.Config, cb ErrCb, logger *log.Logger) (*NSQProducer, error)
```

### func Publish

```go
func (n *NSQProducer) Publish(topic string, body []byte) error
```

### func Close

```go
func (n *NSQProducer) Close() 
```

### func CurStatus

```go
type NSQDStatus struct {
	Addr    string		// 当前节点地址
	Status  bool		// 当前节点状态
	LastErr error 		// 上次出错的原因
}

func (n *NSQProducer) CurStatus() []NSQDStatus
```