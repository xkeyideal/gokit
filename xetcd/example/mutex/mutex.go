package main

import (
	"github.com/xkeyideal/gokit/xetcd/mutex"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:16001", "127.0.0.1:16003", "127.0.0.1:16005"},
		Username:  "root",
		Password:  "root",
	})

	if err != nil {
		panic(err)
	}

	mutexKey := "/my-mutex"

	mu, err := mutex.NewEtcdLeaseMutex(mutexKey, etcdClient, 10)
	if err != nil {
		panic(err)
	}
	defer mu.Close()

	err = mu.Lock()
	if err != nil {
		panic(err)
	}
	defer mu.Unlock()

	// todo anything
}
