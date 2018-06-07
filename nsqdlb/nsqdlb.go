package nsqdlb

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	nsq "github.com/nsqio/go-nsq"
)

const (
	NSQDPING_INTERVAL = 60 * time.Second
)

type NSQDStatus struct {
	Addr    string
	Status  bool
	LastErr error
}

type ErrCb func(addr string, err error)

type NSQProducer struct {
	nsqdAddrs []string
	producers []*NsqdConn
	errCb     ErrCb
}

func NewNSQProducer(nsqdAddrs []string, cfg *nsq.Config, cb ErrCb, logger *log.Logger) (*NSQProducer, error) {
	if cfg == nil {
		cfg = nsq.NewConfig()
	}

	n := &NSQProducer{
		nsqdAddrs: nsqdAddrs,
		producers: []*NsqdConn{},
		errCb:     cb,
	}

	for _, addr := range nsqdAddrs {
		nc := newNsqdConn(addr, cfg, logger)
		err := nc.connect()
		if err != nil {
			return nil, err
		}
		n.producers = append(n.producers, nc)
	}

	go n.checkNsqd()

	return n, nil
}

func (n *NSQProducer) CurStatus() []NSQDStatus {

	st := []NSQDStatus{}
	l := len(n.producers)

	for i := 0; i < l; i++ {
		addr, ok := n.producers[i].live()
		st = append(st, NSQDStatus{
			Addr:    addr,
			Status:  ok,
			LastErr: n.producers[i].lastErr,
		})
	}

	return st
}

func (n *NSQProducer) checkNsqd() {

	for range time.Tick(NSQDPING_INTERVAL) {
		l := len(n.producers)

		for i := 0; i < l; i++ {

			err := n.producers[i].conn.Ping()
			if err != nil {
				n.producers[i].down(err)
				n.errCb(n.producers[i].addr, err)
			} else {
				n.producers[i].up()
			}
		}
	}
}

func (n *NSQProducer) chooseNsqd() (int, error) {
	l := len(n.producers)

	liveIndexes := []int{}

	for i := 0; i < l; i++ {
		if _, ok := n.producers[i].live(); ok {
			liveIndexes = append(liveIndexes, i)
		}
	}

	if len(liveIndexes) == 0 {
		return -1, fmt.Errorf("Time: %v, all nsdqs are dead", time.Now())
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(liveIndexes))

	return liveIndexes[index], nil
}

func (n *NSQProducer) Publish(topic string, body []byte) error {
	index, err := n.chooseNsqd()
	if err != nil {
		return err
	}

	return n.producers[index].conn.Publish(topic, body)
}

func (n *NSQProducer) Close() {
	l := len(n.producers)

	for i := 0; i < l; i++ {
		n.producers[i].close()
	}
}
