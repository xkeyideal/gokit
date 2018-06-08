package tredis

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xkeyideal/gokit/httpkit"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

const (
	pingDuration      = 10 * time.Second
	httpRwTimeout     = 5               // 读写超时5秒
	httpConnTimout    = 2 * time.Second //连接超时2秒
	httpRetry         = 3               // 重试1次
	httpRetryInterval = 2 * time.Second //重试间隔2秒
)

type ErrCb func(err error)

type QueryTRedisAddrsCb func(url, project, env string, client *httpkit.HttpClient) ([]string, error)

type TRedisWatchClient struct {
	wsurl              string
	httpurl            string
	cmdName            string
	watchKey           string
	uuid               string
	env                string
	projects           map[string]struct{}
	conn               *websocket.Conn
	ticker             *time.Ticker
	sendChan           chan []byte
	exitChan           chan struct{}
	wg                 sync.WaitGroup
	httpClient         *httpkit.HttpClient
	errCb              ErrCb
	queryTRedisAddrsCb QueryTRedisAddrsCb

	ChangeChan chan *TRedisWatchResp
}

type TRedisWatchResp struct {
	Ips     []string
	Project string
	Err     error
}

type writeMessageProtocol struct {
	CmdId   string   `json:"cmdId"`
	CmdName string   `json:"cmdName"`
	Env     string   `json:"env"`
	Args    []string `json:"args"`
}

type readMessageProtocol struct {
	CmdId       string `json:"cmdId"`
	CmdName     string `json:"cmdName"`
	ReturnValue string `json:"returnValue"`
}

func NewTRedisWatchClient(wsurl, cmdName, watchKey, env, httpurl string, projects []string, cb ErrCb, qcb QueryTRedisAddrsCb) (*TRedisWatchClient, error) {
	fmt.Println(wsurl)
	conn, _, err := websocket.DefaultDialer.Dial(wsurl, nil)
	if err != nil {
		return nil, err
	}

	client := &TRedisWatchClient{
		conn:               conn,
		wsurl:              wsurl,
		projects:           make(map[string]struct{}),
		cmdName:            cmdName,
		watchKey:           watchKey,
		env:                env,
		httpurl:            httpurl,
		uuid:               uuid.NewV4().String(),
		sendChan:           make(chan []byte, 2),
		exitChan:           make(chan struct{}),
		ticker:             time.NewTicker(pingDuration),
		ChangeChan:         make(chan *TRedisWatchResp, len(projects)),
		errCb:              cb,
		queryTRedisAddrsCb: qcb,

		httpClient: httpkit.NewHttpClient(httpRwTimeout, httpRetry, httpRetryInterval, httpConnTimout, nil),
	}

	args := []string{}
	for _, project := range projects {
		client.projects[project] = struct{}{}
		args = append(args, fmt.Sprintf("%s:%s", project, client.watchKey))
	}

	w := writeMessageProtocol{
		CmdId:   client.uuid,
		CmdName: client.cmdName,
		Env:     client.env,
		Args:    args,
	}

	b, _ := json.Marshal(w)

	client.sendChan <- b

	go client.write() // write只需要定时发送Ping即可
	go client.read()  // 监听server告知发送变化的project

	return client, nil
}

func (client *TRedisWatchClient) DelProject(project string) {
	if _, ok := client.projects[project]; !ok {
		return
	}

	if len(client.projects) <= 1 {
		return
	}

	delete(client.projects, project)

	args := []string{}
	for project := range client.projects {
		args = append(args, fmt.Sprintf("%s:%s", project, client.watchKey))
	}

	w := writeMessageProtocol{
		CmdId:   client.uuid,
		CmdName: client.cmdName,
		Env:     client.env,
		Args:    args,
	}

	b, _ := json.Marshal(w)

	client.sendChan <- b

	return
}

func (client *TRedisWatchClient) AddProject(project string) {
	if _, ok := client.projects[project]; ok {
		return
	}

	client.projects[project] = struct{}{}

	args := []string{}
	for project := range client.projects {
		args = append(args, fmt.Sprintf("%s:%s", project, client.watchKey))
	}

	w := writeMessageProtocol{
		CmdId:   client.uuid,
		CmdName: client.cmdName,
		Env:     client.env,
		Args:    args,
	}

	b, _ := json.Marshal(w)

	client.sendChan <- b

	return
}

func (client *TRedisWatchClient) Close() {
	client.ticker.Stop()
	close(client.exitChan)
	client.wg.Wait()
	client.conn.Close()
}

func (client *TRedisWatchClient) read() {

	client.wg.Add(1)
	for {
		select {
		case <-client.exitChan:
			goto exit
		default:
			typ, message, err := client.conn.ReadMessage()
			if typ != websocket.TextMessage && typ != websocket.BinaryMessage {
				continue
			}

			if err != nil {
				client.errCb(err)
			}

			r := readMessageProtocol{}
			err = json.Unmarshal(message, &r)
			if err != nil {
				continue
			}

			if strings.ToLower(r.ReturnValue) == "ok" {
				continue
			}

			ss := strings.Split(r.ReturnValue, ":")
			if len(ss) == 2 {
				if r.CmdName == "projectkeyconfigchange" && ss[1] == client.watchKey {
					project := ss[0]
					url := fmt.Sprintf(client.httpurl, client.env, project)
					ips, err := client.queryTRedisAddrsCb(url, project, client.env, client.httpClient)

					select {
					case client.ChangeChan <- &TRedisWatchResp{
						Ips:     ips,
						Project: project,
						Err:     err,
					}:
					default:
						continue
					}
				}
			}
		}
	}

exit:
	client.wg.Done()
	fmt.Println("tredis watch read exit")
}

func (client *TRedisWatchClient) write() {

	client.wg.Add(1)

	for {
		select {
		case <-client.exitChan:
			client.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			goto exit
		case <-client.ticker.C:
			err := client.conn.WriteMessage(websocket.PingMessage, []byte{})
			if err != nil {
				client.errCb(err)
			}
		case message := <-client.sendChan:
			fmt.Println(string(message))
			client.conn.WriteMessage(websocket.TextMessage, message)
		}
	}

exit:
	client.wg.Done()
	fmt.Println("tredis watch write exit")
}
