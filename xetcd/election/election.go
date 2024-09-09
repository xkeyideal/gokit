package election

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/xkeyideal/gokit/xetcd/concurrency"

	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	reconnectBackOff = time.Second * 2
	session          *concurrency.Session
	election         *concurrency.Election
)

type ElectLeader struct {
	ttl int

	isLeader bool
	leaderCh chan bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewElectLeader(ttl int) *ElectLeader {
	ctx, cancel := context.WithCancel(context.Background())
	return &ElectLeader{
		ttl:      ttl,
		isLeader: false,
		leaderCh: make(chan bool, 10),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (el *ElectLeader) ElectLeader(client *clientv3.Client, host, electKey string) error {
	if err := el.newSession(client, 0, electKey); err != nil {
		log.Printf("while creating new session: %s", err)
		return err
	}

	go el.electLoop(client, host, electKey)

	for {
		resp, err := election.Leader(el.ctx)
		if err != nil {
			if err != concurrency.ErrElectionNoLeader {
				return err
			}
			time.Sleep(time.Millisecond * 300)
			continue
		}

		if string(resp.Kvs[0].Value) != host {
			el.leaderCh <- false
		}
		break
	}

	return nil
}

func (el *ElectLeader) electLoop(client *clientv3.Client, host, electKey string) error {
	var (
		errCh   chan error
		node    *clientv3.GetResponse
		observe <-chan clientv3.GetResponse
		err     error
		recnt   int
	)

	el.wg.Add(1)
	defer el.wg.Done()

	for {
		// 在尝试推选自己为leader之前，先查询etcd里是否有leader
		if node, err = election.Leader(el.ctx); err != nil {
			// Leader() 正常的错误值是NoLeader
			if err != concurrency.ErrElectionNoLeader {
				log.Fatalf("while determining election leader: %s", err)
				goto reconnect
			}
		} else {
			log.Printf("query, key:%s, leader: %s, candidate: %s", string(node.Kvs[0].Key), string(node.Kvs[0].Value), host)

			// 当前etcd里显示自己是leader
			if string(node.Kvs[0].Value) == host {
				// 如果需要根据现有的值恢复leader
				if true {
					session.Close() // 先释放之前的session
					// 依据session里old lease，重建自己为主的session
					if err = el.newSession(client, node.Kvs[0].Lease, electKey); err != nil {
						log.Fatalf("while re-establishing session with lease: %s", err)
						goto reconnect
					}
					election = concurrency.ResumeElection(session, electKey,
						string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)

					// Because Campaign() only returns if the election entry doesn't exist
					// we must skip the campaign call and go directly to observe when resuming
					goto observe
				} else {
					// If resign takes longer than our TTL then lease is expired and we are no
					// longer leader anyway.
					// 无需恢复leader，则释放当前自己为leader
					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(el.ttl)*time.Second)
					election := concurrency.ResumeElection(session, electKey,
						string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)
					err = election.Resign(ctx)
					cancel()

					if err != nil {
						log.Fatalf("while resigning leadership after reconnect: %s", err)
						goto reconnect
					}
				}
			}
		}

		// 选举之前先设置
		el.setLeader(false)

		// 尝试推选自己为leader
		errCh = make(chan error)
		go func() {
			// Make this a non blocking call so we can check for session close
			errCh <- election.Campaign(el.ctx, host)
		}()

		select {
		case err = <-errCh:
			if err != nil {
				if errors.Cause(err) == context.Canceled {
					return err
				}
				// NOTE: Campaign currently does not return an error if session expires
				log.Fatalf("while campaigning for leader: %s", err)
				session.Close()
				goto reconnect
			}
		case <-el.ctx.Done():
			session.Close()
			return err
		case <-session.Done():
			goto reconnect
		}

	observe:
		// If Campaign() returned without error, we are leader
		el.setLeader(true)
		// 观测选主的变化
		observe = election.Observe(el.ctx)
		for {
			select {
			case resp, ok := <-observe:
				log.Printf("observe, key:%s, leader: %s, candidate: %s", string(resp.Kvs[0].Key), string(resp.Kvs[0].Value), host)
				if !ok {
					// NOTE: Observe will not close if the session expires, we must
					// watch for session.Done()
					session.Close()
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
					if err = election.Resign(ctx); err != nil {
						log.Printf("while resigning leadership during shutdown: %s", err)
					}
					cancel()
				}

				session.Close()
				return nil
			case <-session.Done():
				goto reconnect
			}
		}

	reconnect:
		el.setLeader(false)
		for {
			recnt = 0
			session.Close() // 重连之前先释放之前的session
			if err = el.newSession(client, 0, electKey); err != nil {
				recnt++
				log.Printf("while creating new session: %s", err)
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
	var err error
	session, err = concurrency.NewSession(
		client,
		concurrency.WithLease(clientv3.LeaseID(id)),
		concurrency.WithTTL(el.ttl),
		concurrency.WithContext(el.ctx),
	)
	if err != nil {
		return err
	}

	election = concurrency.NewElection(session, electKey)

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
	resp, err := election.Leader(context.Background())

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
