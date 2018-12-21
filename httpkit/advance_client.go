package httpkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"
)

type AdvanceHttpClient struct {
	host   string
	scheme string
	client *http.Client
}

func NewAdvanceHttpClient(scheme, host string, connTimeout time.Duration, tlsCfg *tls.Config) *AdvanceHttpClient {
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   connTimeout,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,

		TLSClientConfig:       tlsCfg,
		DisableKeepAlives:     false,
		MaxIdleConns:          500,
		MaxIdleConnsPerHost:   300,
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

	return &AdvanceHttpClient{
		scheme: scheme,
		host:   host,
		client: client,
	}
}

type AdvanceSettings struct {
	readWriteTimeout time.Duration
	path             string
	params           url.Values
	headers          http.Header
	cookie           *http.Cookie
	body             io.Reader
	rawBody          []byte
	baseAuth         bool
	baseAuthUsername string
	baseAuthPassword string
	gzip             bool
	retry            int
	retryInterval    time.Duration
}

type AdvanceResponse struct {
	Body       []byte
	Header     http.Header
	StatusCode int
	Status     string
	Time       int64
}

func NewAdvanceSettings(rwTimeout time.Duration, retry int, retryInterval time.Duration) *AdvanceSettings {
	return &AdvanceSettings{
		readWriteTimeout: rwTimeout,
		params:           url.Values{},
		headers:          http.Header{},
		baseAuth:         false,
		gzip:             false,
		retry:            retry,
		retryInterval:    retryInterval,
	}
}

func (setting *AdvanceSettings) urlString(scheme, host, path string) string {
	urlPath := url.URL{
		Scheme:   scheme,
		Host:     host,
		Path:     path,
		RawQuery: setting.params.Encode(),
	}

	return urlPath.String()
}

func (setting *AdvanceSettings) EnableGZip(gzip bool) *AdvanceSettings {
	setting.gzip = gzip
	return setting
}

func (setting *AdvanceSettings) SetParam(key, value string) *AdvanceSettings {
	setting.params.Set(key, value)
	return setting
}

func (setting *AdvanceSettings) AddParam(key, value string) *AdvanceSettings {
	setting.params.Add(key, value)
	return setting
}

func (setting *AdvanceSettings) SetParams(kvs map[string]string) *AdvanceSettings {
	for key, value := range kvs {
		setting.params.Set(key, value)
	}

	return setting
}

func (setting *AdvanceSettings) SetHeader(key, value string) *AdvanceSettings {
	setting.headers.Set(key, value)
	return setting
}

func (setting *AdvanceSettings) AddHeader(key, value string) *AdvanceSettings {
	setting.headers.Add(key, value)
	return setting
}

func (setting *AdvanceSettings) SetHeaders(kvs map[string]string) *AdvanceSettings {
	for key, value := range kvs {
		setting.headers.Set(key, value)
	}

	return setting
}

func (setting *AdvanceSettings) SetCookie(cookie *http.Cookie) *AdvanceSettings {
	setting.cookie = cookie
	return setting
}

func (setting *AdvanceSettings) SetBody(body io.Reader) *AdvanceSettings {
	rawBody, _ := ioutil.ReadAll(body)
	setting.body = bytes.NewBuffer(rawBody)
	setting.rawBody = rawBody
	return setting
}

func (setting *AdvanceSettings) SetBasicAuth(username, password string) *AdvanceSettings {
	setting.baseAuth = true
	setting.baseAuthUsername = username
	setting.baseAuthPassword = password
	return setting
}

func (client *AdvanceHttpClient) Get(uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	setting.body = nil
	setting.rawBody = []byte{}
	return client.do("GET", uri, setting)
}

func (client *AdvanceHttpClient) Post(uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	return client.do("POST", uri, setting)
}

func (client *AdvanceHttpClient) Put(uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	return client.do("PUT", uri, setting)
}

func (client *AdvanceHttpClient) Delete(uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	return client.do("DELETE", uri, setting)
}

func (client *AdvanceHttpClient) Head(uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	return client.do("HEAD", uri, setting)
}

func (client *AdvanceHttpClient) Do(method, targetUrl string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	return client.do(method, targetUrl, setting)
}

func genHttpRequest(method, url string, setting *AdvanceSettings) (*http.Request, error) {
	req, err := http.NewRequest(method, url, setting.body)
	if err != nil {
		return nil, err
	}

	for key, values := range setting.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	if setting.cookie != nil {
		req.Header.Add("Cookie", setting.cookie.String())
	}

	if setting.baseAuth {
		req.SetBasicAuth(setting.baseAuthUsername, setting.baseAuthPassword)
	}

	return req, nil
}

func (client *AdvanceHttpClient) do(method, uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, err
	}

	if u.RawQuery != "" {
		return nil, fmt.Errorf("uri中不能存在query参数[%s]，请使用setting.SetParam等方法预设置", u.RawQuery)
	}

	url := setting.urlString(client.scheme, client.host, uri)

	var ctx context.Context
	var cancel context.CancelFunc
	var resp *http.Response
	var err2 error

	if setting.retry <= 0 {
		setting.retry = 1
	} else {
		setting.retry++
	}

	startTime := time.Now()
	for i := 0; i < setting.retry; i++ {
		// solve Golang http post error : http: ContentLength=355 with Body length 0 bug
		req, err := genHttpRequest(method, url, setting)
		if err != nil {
			return nil, err
		}

		if setting.readWriteTimeout > 0 {
			ctx, cancel = context.WithTimeout(req.Context(), setting.readWriteTimeout)
			req = req.WithContext(ctx)
		}

		resp, err2 = client.client.Do(req)

		if cancel != nil {
			cancel()
		}

		if err2 != nil {
			time.Sleep(setting.retryInterval)
			setting.body = bytes.NewBuffer(setting.rawBody)
			continue
		}

		// 非2xx 或 3xx的状态码也认为是服务端响应出错，需重试
		if !(resp.StatusCode >= 200 && resp.StatusCode < 400) {
			time.Sleep(setting.retryInterval)
			setting.body = bytes.NewBuffer(setting.rawBody)
			continue
		}

		break
	}

	if err2 != nil {
		return nil, err2
	}

	defer resp.Body.Close()

	adresp := &AdvanceResponse{
		Header:     resp.Header,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}

	if setting.gzip && resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}

		body, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, err
		}

		adresp.Body = body
		adresp.Time = int64(time.Now().Sub(startTime))

		return adresp, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	adresp.Body = body
	adresp.Time = int64(time.Now().Sub(startTime))

	return adresp, nil
}
