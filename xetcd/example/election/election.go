package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xkeyideal/gokit/xetcd/election"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	if len(os.Args) <= 1 {
		fmt.Println("input arg $1 leader candidate")
		os.Exit(1)
	}

	host := os.Args[1]

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"127.0.0.1:16001", "127.0.0.1:16003", "127.0.0.1:16005"},
		Username:  "root",
		Password:  "root",
	})

	if err != nil {
		panic(err)
	}

	electKey := "/my-election"

	el := election.NewElectLeader(15, false)
	err = el.ElectLeader(etcdClient, host, electKey)
	if err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT)
	go func() {
		for {
			select {
			case <-c:
				fmt.Printf("Resign election and exit\n")
				el.Close()
			}
		}
	}()

	for leader := range el.LeaderCh() {
		log.Println(leader)
	}
}
