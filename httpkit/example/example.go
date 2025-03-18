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

	//resp, err := otelhttp.Post(ctx, "http://127.0.0.1:12745/test", "", bytes.NewBuffer(jsonstr))

	// req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:12745/test", bytes.NewBuffer(jsonstr))
	// if err != nil {
	// 	panic(err)
	// }

	// resp, err := DefaultClient.Do(req)

	resp, err := client.PostWithContext(ctx, "http://127.0.0.1:12745/test")
	if err != nil {
		fmt.Println(err)
		return
	}

	//defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("uri error: %s, code: %d", resp.Status, resp.StatusCode)
		return
	}

	//body, _ := io.ReadAll(resp.Body)

	fmt.Println(string(resp.Body))
}
