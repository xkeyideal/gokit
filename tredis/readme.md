# nsqdlb
    import "github.com/xkeyideal/gokit/tredis"

1. 主动watch项目redis地址的变化
2. 可以监听多个项目，发生变化后，该kit会主动调取http接口，获取最新的ip地址

## Example

```go
type TRedisWatchResp struct {
	Ips     []string
	Project string
	Err     error
}

func cb(addr string, err error) {
	if err != nil {
		fmt.Println(addr, "Err: ", err)
	}
}

func main() {
	client, err := tredis.NewTRedisWatchClient(wsurl, cmdName, watchKey, env, httpUrl, projects, cb)
	if err != nil {
		fmt.Println("err11: ", err)
		return
	}

	go func() {
		for watchResp := range client.ChangeChan {
            fmt.Println(watchResp)
            if watchResp.Err != nil {
                // do something for update old redis cluster ips
            }
		}
	}()

	//time.Sleep(100 * time.Second)
	client.Close()
}

```
