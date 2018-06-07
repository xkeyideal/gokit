package nsqdlb

import (
	"fmt"
	"io/ioutil"
	"log"
	"sync/atomic"
	"time"

	nsq "github.com/nsqio/go-nsq"
)

const (
	DOWN = 1
	UP   = 2
)

var nullLogger = log.New(ioutil.Discard, "", log.LstdFlags)

type NsqdConn struct {
	conn *nsq.Producer

	cfg    *nsq.Config
	logger *log.Logger

	addr string

	status int32

	lastErr error
}

func newNsqdConn(addr string, cfg *nsq.Config, logger *log.Logger) *NsqdConn {
	return &NsqdConn{
		addr:   addr,
		cfg:    cfg,
		logger: logger,
		status: DOWN,
	}
}

func (nc *NsqdConn) connect() error {
	var err error
	nc.conn, err = nsq.NewProducer(nc.addr, nc.cfg)
	if nc.logger == nil {
		nc.conn.SetLogger(nullLogger, nsq.LogLevelInfo)
	} else {
		nc.conn.SetLogger(nc.logger, nsq.LogLevelInfo)
	}

	if err != nil {
		return fmt.Errorf("Time: %v, nsqd: %s NewProducer Err: %s", time.Now(), nc.addr, err.Error())
	}

	err = nc.conn.Ping()
	if err != nil {
		return fmt.Errorf("Time: %v, nsqd: %s Ping Err: %s", time.Now(), nc.addr, err.Error())
	}

	atomic.StoreInt32(&(nc.status), UP)

	return nil
}

func (nc *NsqdConn) live() (string, bool) {
	return nc.addr, atomic.LoadInt32(&(nc.status)) == UP
}

func (nc *NsqdConn) up() {
	if atomic.LoadInt32(&(nc.status)) == UP {
		return
	}

	nc.lastErr = nc.connect()
}

func (nc *NsqdConn) down(err error) {
	nc.lastErr = fmt.Errorf("Time: %v, nsqd: %s Ping Err: %s", time.Now(), nc.addr, err.Error())
	if atomic.LoadInt32(&(nc.status)) == DOWN {
		return
	}

	atomic.StoreInt32(&(nc.status), DOWN)
}

func (nc *NsqdConn) close() {
	if nc.conn != nil {
		nc.conn.Stop()
	}
}
