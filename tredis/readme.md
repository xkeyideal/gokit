# tredis

    import "github.com/xkeyideal/gokit/tredis"

1. 主动watch项目redis地址的变化
2. 可以监听多个项目，发生变化后，该kit会主动调取http接口，获取最新的ip地址
3. 由于涉及安全问题，该包缺少一个request.go文件，该文件中提供了一个查询redis ip地址的接口和各类结构体的定义,若需使用，请加上该文件或自行实现该回调函数

```go
// 提供查询`project`的URL地址，根据指定的环境返回redis ip地址
type QueryTRedisAddrsCb func(url, project, env string, client *httpkit.HttpClient) ([]string, error)
```

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

func qcb(url, project, env string, client *httpkit.HttpClient) ([]string, error) {
	resp, err := client.SetBasicAuth(project, project).Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("uri error: %s, code: %d", resp.Status, resp.StatusCode)
	}

	// do something for resolving redis ip addresses

	return []string{"127.0.0.1:6379","127.0.0.1:6380","127.0.0.1:6381"}, nil
}

func main() {
	client, err := tredis.NewTRedisWatchClient(wsurl, cmdName, watchKey, env, httpUrl, projects, cb, qcb)
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
