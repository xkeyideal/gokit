package httpkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

type HttpClient struct {
	c                *http.Client
	rwTimeout        time.Duration
	params           url.Values
	headers          http.Header
	cookie           *http.Cookie
	body             io.Reader
	rawBody          []byte //原始body备份使用，retry的时候使用
	baseAuth         bool
	baseAuthUsername string
	baseAuthPassword string
	gzip             bool
	retry            int
	retryInterval    time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

func NewHttpClient(rwTimeout time.Duration, retry int,
	retryInterval, connTimeout time.Duration, tlsCfg *tls.Config) *HttpClient {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   connTimeout,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,

		TLSClientConfig:       tlsCfg,
		DisableKeepAlives:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &HttpClient{
		c:             client,
		params:        url.Values{},
		headers:       http.Header{},
		rwTimeout:     rwTimeout,
		baseAuth:      false,
		gzip:          false,
		retry:         retry,
		retryInterval: retryInterval,
	}
}

func (client *HttpClient) EnableGZip(gzip bool) *HttpClient {
	client.gzip = gzip
	return client
}

func (client *HttpClient) SetParam(key, value string) *HttpClient {
	client.params.Set(key, value)

	return client
}

func (client *HttpClient) AddParam(key, value string) *HttpClient {
	client.params.Add(key, value)
	return client
}

func (client *HttpClient) AddParams(params url.Values) *HttpClient {
	client.params = params
	return client
}

func (client *HttpClient) SetParams(kvs map[string]string) *HttpClient {
	for key, value := range kvs {
		client.params.Set(key, value)
	}

	return client
}

func (client *HttpClient) SetHeader(key, value string) *HttpClient {
	client.headers.Set(key, value)
	return client
}

func (client *HttpClient) AddHeader(key, value string) *HttpClient {
	client.headers.Add(key, value)
	return client
}

func (client *HttpClient) SetHeaders(kvs map[string]string) *HttpClient {
	for key, value := range kvs {
		client.headers.Set(key, value)
	}

	return client
}

func (client *HttpClient) AddHeaders(headers http.Header) *HttpClient {
	client.headers = headers
	return client
}

func (client *HttpClient) SetCookie(cookie *http.Cookie) *HttpClient {
	client.cookie = cookie
	return client
}

func (client *HttpClient) SetBasicAuth(username, password string) *HttpClient {
	client.baseAuth = true
	client.baseAuthUsername = username
	client.baseAuthPassword = password
	return client
}

func (client *HttpClient) SetBody(body io.Reader) *HttpClient {
	rawBody, _ := ioutil.ReadAll(body)
	client.body = bytes.NewBuffer(rawBody)
	client.rawBody = rawBody
	return client
}

func (client *HttpClient) Get(targetUrl string) (*AdvanceResponse, error) {
	client.body = nil
	client.rawBody = []byte{}
	return client.do("GET", targetUrl)
}

func (client *HttpClient) Post(targetUrl string) (*AdvanceResponse, error) {
	return client.do("POST", targetUrl)
}

func (client *HttpClient) Put(targetUrl string) (*AdvanceResponse, error) {
	return client.do("PUT", targetUrl)
}

func (client *HttpClient) Delete(targetUrl string) (*AdvanceResponse, error) {
	return client.do("DELETE", targetUrl)
}

func (client *HttpClient) Head(targetUrl string) (*AdvanceResponse, error) {
	return client.do("HEAD", targetUrl)
}

func (client *HttpClient) Do(method, targetUrl string) (*AdvanceResponse, error) {
	return client.do(method, targetUrl)
}

func (client *HttpClient) genHttpRequest(method, targetUrl string) (*http.Request, error) {
	u, err := url.Parse(targetUrl)
	if err != nil {
		return nil, err
	}

	if u.RawQuery != "" {
		return nil, fmt.Errorf("url中不能存在query参数[%s]，请使用client.SetParam等方法预设置", u.RawQuery)
	}

	u.RawQuery = client.params.Encode()

	req, err := http.NewRequest(method, u.String(), client.body)
	if err != nil {
		return nil, err
	}

	for key, values := range client.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if client.cookie != nil {
		req.Header.Add("Cookie", client.cookie.String())
	}

	if client.baseAuth {
		req.SetBasicAuth(client.baseAuthUsername, client.baseAuthPassword)
	}

	return req, nil
}

func (client *HttpClient) do(method, targetUrl string) (*AdvanceResponse, error) {

	if client.retry <= 0 {
		client.retry = 1
	} else {
		client.retry++
	}

	adresp := &AdvanceResponse{}

	startTime := time.Now()
	for i := 0; i < client.retry; i++ {
		err := client.doOnce(method, targetUrl, adresp)
		if err != nil {
			time.Sleep(client.retryInterval)
			client.body = bytes.NewBuffer(client.rawBody)
			continue
		}
		break
	}

	adresp.Time = int64(time.Now().Sub(startTime))

	return adresp, nil
}

func (client *HttpClient) doOnce(method, targetUrl string, adresp *AdvanceResponse) error {

	req, err := client.genHttpRequest(method, targetUrl)
	if err != nil {
		return err
	}

	if client.rwTimeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), client.rwTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := client.c.Do(req)

	if err != nil {
		//time.Sleep(client.retryInterval)
		//client.body = bytes.NewBuffer(client.rawBody)
		return err
	}

	defer resp.Body.Close()

	// 非2xx 或 3xx的状态码也认为是服务端响应出错，需重试
	if !(resp.StatusCode >= 200 && resp.StatusCode < 400) {
		//time.Sleep(client.retryInterval)
		//client.body = bytes.NewBuffer(client.rawBody)
		return errors.New("status code error")
	}

	adresp.Header = resp.Header
	adresp.StatusCode = resp.StatusCode
	adresp.Status = resp.Status

	if client.gzip && resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}

		body, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}

		adresp.Body = body
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	adresp.Body = body
	return nil
}
