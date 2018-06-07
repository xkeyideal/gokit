package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xkeyideal/gokit/httpkit"
)

var (
	Username = "xxxxx"
	Password = "123456789"

	RwTimeout     = 5               // 读写超时5秒
	ConnTimout    = 2 * time.Second //连接超时2秒
	Retry         = 1               // 重试1次
	RetryInterval = 2 * time.Second //重试间隔2秒

	Url = "http://www.baidu.com"
)

type PostBodyExample struct {
	A string   `json:"a"`
	B []string `json:"b"`
	C bool     `json:"c"`
	D int      `json:"d"`
}

type PostResp struct {
	Resp string `json:"resp"`
}

func SimpleClientExample() ([]PostResp, error) {
	client := httpkit.NewHttpClient(RwTimeout, Retry, RetryInterval, ConnTimout, nil)

	jsonstr, _ := json.Marshal(PostBodyExample{
		A: "a",
		B: []string{"b"},
		C: false,
		D: 7,
	})

	client = client.SetHeader("Content-Type", "application/json").SetHeader("Auth-Token", "private-token")

	resp, err := client.SetBody(bytes.NewBuffer(jsonstr)).Post(Url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("uri error: %s, code: %d", resp.Status, resp.StatusCode)
	}

	infos := []PostResp{}
	err = json.Unmarshal(resp.Body, &infos)
	if err != nil {
		return nil, err
	}

	return infos, nil
}
