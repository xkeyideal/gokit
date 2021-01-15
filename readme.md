# gokit

一些个人使用的golang 工具包，主要包括：

1. httpkit http短连接和长连接的客户端
2. tclog 简单的异步日志库
3. mongo mongodb的连接库(已废弃，不建议使用，建议使用官方driver)
4. nsqdlb nsq发送方多个nsqd的负载均衡库
5. tools 常用的一些工具函数
6. tredis 服务发现redis集群ip地址变化
7. xetcd etcd分布式锁与选主

## install

`go get github.com/xkeyideal/gokit`

## xetcd

由于etcd官方golang客户端的包管理实在太过糟糕，官方的concurrency始终存在包的类型冲突，特此将3.4+版本的concurrency包拷贝出。
期待官方早日出3.5版本的客户端

使用方法详见`xetcd/example`目录

### election

由于etcd集群的选主或网络问题会导致`Lease.KeepAlive`被cancel掉，那么lease可能会”消失“，因此需要实现重试逻辑，因此需要处理concurrency里`Done()`的channel。

同时`Campaign()`方法里会执行watch，而`Resign()`方法并不会主动关闭此watch，若出现leader不断切换，会导致watch goroutine泄漏，直接的表现就是etcd的watch连接数
不断的增加，在`election`中会主动处理此情况。

`election`的实现方案，参考了网上提供的方案：`https://gist.github.com/thrawn01/c007e6a37b682d3899910e33243a3cdc`

### mutex

与`election`类似，`mutex`也会执行watch，同样`Unlock()`方法仅仅会执行key的删除，而不会主动关闭watch，`mutex`中会主动处理此情况。

Lucky for use it