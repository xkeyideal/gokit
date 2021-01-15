package mutex

import (
	"context"

	"github.com/xkeyideal/gokit/xetcd/concurrency"

	"go.etcd.io/etcd/clientv3"
)

type EtcdMutex struct {
	ctx     context.Context
	cancel  context.CancelFunc
	session *concurrency.Session
	mutex   *concurrency.Mutex
}

// NewEtcdMutex default 10s
func NewEtcdMutex(key string, client *clientv3.Client) (*EtcdMutex, error) {
	session, err := concurrency.NewSession(client)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &EtcdMutex{
		ctx:     ctx,
		cancel:  cancel,
		session: session,
		mutex:   concurrency.NewMutex(session, key),
	}, nil
}

// NewEtcdLeaseMutex ttl
func NewEtcdLeaseMutex(key string, client *clientv3.Client, ttl int64) (*EtcdMutex, error) {
	res, err := client.Grant(context.Background(), ttl)
	if err != nil {
		return nil, err
	}

	session, err := concurrency.NewSession(client, concurrency.WithLease(res.ID))
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &EtcdMutex{
		ctx:     ctx,
		cancel:  cancel,
		session: session,
		mutex:   concurrency.NewMutex(session, key),
	}, nil
}

//Lock get lock
func (em *EtcdMutex) Lock() error {
	return em.mutex.Lock(em.ctx)
}

func (em *EtcdMutex) Unlock() error {
	err := em.mutex.Unlock(em.ctx)

	return err
}

func (em *EtcdMutex) Close() {
	em.cancel()
	em.session.Close()
}
