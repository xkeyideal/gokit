package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xkeyideal/gokit/httpkit"
)

const (
	rwTimeout     = 5 * time.Second // 读写超时5秒
	connTimout    = 2 * time.Second //连接超时2秒
	retry         = 2               // 重试1次
	retryInterval = 2 * time.Second //重试间隔2秒
)

type TestData struct {
	Name string `json:"name"`
}

func main() {
	client := httpkit.NewHttpClient(rwTimeout, retry, retryInterval, connTimout, nil)

	jsonstr, _ := json.Marshal(TestData{
		Name: "a",
	})

	client = client.SetHeader("Content-Type", "application/json").SetHeader("Auth-Token", "private-token")

	resp, err := client.SetBody(bytes.NewBuffer(jsonstr)).Post("http://10.101.44.49:12745/test")
	if err != nil {
		fmt.Println(err)
		return
	}

	if resp.StatusCode != 200 {
		fmt.Printf("uri error: %s, code: %d", resp.Status, resp.StatusCode)
		return
	}

	fmt.Println(string(resp.Body))
}
