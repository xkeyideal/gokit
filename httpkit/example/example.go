package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/xkeyideal/gokit/httpkit"
)

const (
	rwTimeout     = 5 * time.Second // 读写超时5秒
	connTimout    = 2 * time.Second //连接超时2秒
	retry         = 2               // 重试1次
	retryInterval = 2 * time.Second //重试间隔2秒
)

var (
	retryHttpStatuses = []int{400, 401}
)

type TestData struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var DefaultClient = &http.Client{}

func main() {
	client := httpkit.NewHttpClient(rwTimeout, retry, retryInterval, connTimout, nil, retryHttpStatuses...)

	jsonstr, _ := json.Marshal(TestData{
		Name:  "abv",
		Email: "1111@gmail.com",
	})

	client = client.SetHeader("Content-Type", "application/json").SetHeader("Auth-Token", "private-token")
	client = client.SetBody(bytes.NewBuffer(jsonstr))

	curl, err := client.ToCurlCommand(context.Background(), http.MethodPost, "http://127.0.0.1:12745/test")
	if err != nil {
		panic(err)
	}

	log.Println(curl)

	ctx := context.WithValue(context.Background(), "req.withcontext", "123456")

	params := map[string]string{
		"key":  "key",
		"from": "from",
		"to":   "to",
	}

	b, _ := json.Marshal(params)
	client = client.SetParam("mode", "driving")
	client = client.SetHeader("Content-Type", "application/json")
	client = client.SetBody(bytes.NewBuffer(b))

	resp, err := client.PostWithContext(ctx, "http://127.0.0.1:12745/test")
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
