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

	// 初次启动, 查询etcd自己是Leader后，是否依然重新选举, 默认是false(不重新选举)
	recampaign bool

	electKey string // 参与选举的key
	isLeader bool
	leaderCh chan bool

	session  *concurrency.Session
	election *concurrency.Election

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewElectLeader(ttl int, recampaign bool, electKey string) *ElectLeader {
	ctx, cancel := context.WithCancel(context.Background())

	return &ElectLeader{
		ttl:        ttl,
		recampaign: recampaign,
		electKey:   electKey,
		isLeader:   false,
		leaderCh:   make(chan bool, 1),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (el *ElectLeader) ElectLeader(client *clientv3.Client, host string) error {
	err := el.newSession(client, 0, el.electKey)
	if err != nil {
		return err
	}

	go el.electLoop(client, host)

	// 只有el.recampaign == true的时候, 才检测当前的主，否则等待electLoop的选举
	// before election check if had a leader
	if el.recampaign {
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
				el.leaderCh <- false
			} else {
				if !el.recampaign {
					el.leaderCh <- true
				}
			}

			break
		}
	}

	return nil
}

func (el *ElectLeader) electLoop(client *clientv3.Client, host string) error {
	var (
		errCh       chan error
		node        *clientv3.GetResponse
		observe     <-chan clientv3.GetResponse
		err         error
		recnt       int
		notYetElect bool = true
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
				if !el.recampaign {
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
				} else {
					// 尚未选举，且el.recampaign == true, 才释放掉自身的leader
					if notYetElect {
						// If resign takes longer than our TTL then lease is expired and we are no
						// longer leader anyway.
						// 无需恢复leader，则释放当前自己为leader
						el.election = concurrency.ResumeElection(
							el.session, el.electKey,
							string(node.Kvs[0].Key), node.Kvs[0].CreateRevision,
						)

						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(el.ttl)*time.Second)
						err = el.election.Resign(ctx)
						cancel()

						if err != nil {
							log.Printf("while resigning leadership after reconnect: %s\n", err)
							goto reconnect
						}
					}
				}
			}
		}

		notYetElect = false
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
		observe = el.election.Observe(el.ctx)
		for {
			select {
			case resp, ok := <-observe:
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
