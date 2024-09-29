package election

import (
	"context"
	"log"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3/concurrency"

	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	reconnectBackOff = time.Second * 2
)

type ElectLeader struct {
	ttl int

	electKey string // 参与选举的key
	isLeader bool
	leaderCh chan bool

	session  *concurrency.Session
	election *concurrency.Election

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewElectLeader(ttl int, electKey string) *ElectLeader {
	ctx, cancel := context.WithCancel(context.Background())

	return &ElectLeader{
		ttl:      ttl,
		isLeader: false,
		leaderCh: make(chan bool, 10),
		ctx:      ctx,
		cancel:   cancel,
		electKey: electKey,
	}
}

func (el *ElectLeader) ElectLeader(client *clientv3.Client, host string) error {
	err := el.newSession(client, 0, el.electKey)
	if err != nil {
		return err
	}

	go el.electLoop(client, host)

	for {
		resp, err := el.election.Leader(el.ctx)
		if err != nil {
			if err != concurrency.ErrElectionNoLeader {
				return err
			}
			time.Sleep(time.Millisecond * 300)
			continue
		}

		// check if the leader is itself
		if string(resp.Kvs[0].Value) != host {
			el.setLeader(false)
		} else {
			el.setLeader(true)
		}

		break
	}

	return nil
}

func (el *ElectLeader) electLoop(client *clientv3.Client, host string) error {
	var (
		errCh     chan error
		node      *clientv3.GetResponse
		observeCh <-chan clientv3.GetResponse
		err       error
		recnt     int
	)

	el.wg.Add(1)
	defer el.wg.Done()

	for {
		// 在尝试推选自己为leader之前，先查询etcd里是否有leader
		if node, err = el.election.Leader(el.ctx); err != nil {
			// Leader() 正常的错误值是NoLeader
			if err != concurrency.ErrElectionNoLeader {
				goto reconnect
			}
		} else {
			log.Printf("query, key:%s, leader: %s, candidate: %s\n", string(node.Kvs[0].Key), string(node.Kvs[0].Value), host)

			// 当前etcd里显示自己是leader
			if string(node.Kvs[0].Value) == host {
				// 如果需要根据现有的值恢复leader
				el.session.Close() // 先释放之前的session

				// 依据session里old lease，重建自己为主的session
				if err = el.newSession(client, node.Kvs[0].Lease, el.electKey); err != nil {
					log.Printf("while re-establishing session with lease: %s\n", err)
					goto reconnect
				}

				el.election = concurrency.ResumeElection(
					el.session, el.electKey,
					string(node.Kvs[0].Key), node.Kvs[0].CreateRevision,
				)

				// Because Campaign() only returns if the election entry doesn't exist
				// we must skip the campaign call and go directly to observe when resuming
				goto observe
			}
		}

		// 选举之前先设置
		el.setLeader(false)

		// 尝试推选自己为leader
		errCh = make(chan error)
		go func() {
			// Make this a non blocking call so we can check for session close
			errCh <- el.election.Campaign(el.ctx, host)
		}()

		select {
		case err = <-errCh:
			if err != nil {
				if errors.Cause(err) == context.Canceled {
					return err
				}
				// NOTE: Campaign currently does not return an error if session expires
				log.Printf("while campaigning for leader: %s\n", err)
				el.session.Close()
				goto reconnect
			}
		case <-el.ctx.Done():
			el.session.Close()
			return err
		case <-el.session.Done():
			goto reconnect
		}

	observe:
		// If Campaign() returned without error, we are leader
		el.setLeader(true)
		// 观测选主的变化
		observeCh = el.election.Observe(el.ctx)
		for {
			select {
			case resp, ok := <-observeCh:
				log.Printf("observe, key:%s, leader: %s, candidate: %s\n", string(resp.Kvs[0].Key), string(resp.Kvs[0].Value), host)
				if !ok {
					// NOTE: Observe will not close if the session expires, we must
					// watch for session.Done()
					el.session.Close()
					goto reconnect
				}
				if string(resp.Kvs[0].Value) == host {
					el.setLeader(true)
				} else {
					// We are not leader
					el.setLeader(false)
				}
			case <-el.ctx.Done():
				// 选举需退出，如果是leader，则需要释放资源
				if el.isLeader {
					// If resign takes longer than our TTL then lease is expired and we are no
					// longer leader anyway.
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(el.ttl)*time.Second)
					if err = el.election.Resign(ctx); err != nil {
						log.Printf("while resigning leadership during shutdown: %s\n", err)
					}
					cancel()
				}

				el.session.Close()
				return nil
			case <-el.session.Done():
				goto reconnect
			}
		}

	reconnect:
		el.setLeader(false)
		for {
			recnt = 0
			el.session.Close() // 重连之前先释放之前的session
			if err = el.newSession(client, 0, el.electKey); err != nil {
				recnt++
				log.Printf("while creating new session: %s\n", err)
				if errors.Cause(err) == context.Canceled {
					return err
				}

				select {
				case <-el.ctx.Done():
					return nil
				case <-time.After(reconnectBackOff):
					if recnt >= 5 {
						log.Fatal("while reconnect new session exceed 5times")
						return nil
					}
				}

				continue
			}
			break
		}
	}
}

func (el *ElectLeader) newSession(client *clientv3.Client, id int64, electKey string) error {
	session, err := concurrency.NewSession(
		client,
		concurrency.WithLease(clientv3.LeaseID(id)),
		concurrency.WithTTL(el.ttl),
		concurrency.WithContext(el.ctx),
	)
	if err != nil {
		return err
	}

	el.session = session
	el.election = concurrency.NewElection(session, electKey)

	return nil
}

func (el *ElectLeader) setLeader(leader bool) {
	if el.isLeader == leader {
		return
	}

	el.isLeader = leader
	el.leaderCh <- leader
}

func (el *ElectLeader) Leader() (string, error) {
	resp, err := el.election.Leader(context.Background())
	if err != nil {
		return "", err
	}

	return string(resp.Kvs[0].Value), nil
}

func (el *ElectLeader) LeaderCh() <-chan bool {
	return el.leaderCh
}

func (el *ElectLeader) Close() {
	el.cancel()
	close(el.leaderCh)
	log.Println("elect leader closed")
	el.wg.Wait()
}
