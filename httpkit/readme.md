
# httpkit
    import "github.com/xkeyideal/gokit/httpkit"
	

## Simple Client

### func NewTcLog

``` go
func NewHttpClient(rwTimeout, retry int, retryInterval, connTimeout time.Duration, tlsCfg *tls.Config) *HttpClient
```

初始化http client，设置读写超时、重试次数、重试间隔、连接超时、TLS配置


## Advance Client

```go
func NewAdvanceHttpClient(scheme, host string, connTimeout time.Duration, tlsCfg *tls.Config) *AdvanceHttpClient
```

初始化一个长连接的http client，可以按照host作为map的存储key

```go
func NewAdvanceSettings(rwTimeout, retry int, retryInterval time.Duration) *AdvanceSettings
```

设置http client每次请求的参数

```go
func (client *AdvanceHttpClient) Get(uri string, setting *AdvanceSettings) (*AdvanceResponse, error)
```

Get方法的请求示例
	
## Example

短连接http client 详细参考： example/simple_client.go
长连接http client 详细参考： example/advance_client.go

## Advance Client

需要用户自己管理http client对象，本包实现的思路是使用sync.Map包，详细参考用例