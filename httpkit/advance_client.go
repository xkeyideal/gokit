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

	"github.com/moul/http2curl"
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

func NewAdvanceHttpClientWithTransport(
	scheme, host string, connTimeout time.Duration, tlsCfg *tls.Config,
	transport http.RoundTripper,
) *AdvanceHttpClient {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	if transport != nil {
		client.Transport = transport
	} else {
		client.Transport = &http.Transport{
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
	}

	return &AdvanceHttpClient{
		scheme: scheme,
		host:   host,
		client: client,
	}
}

type AdvanceSettings struct {
	readWriteTimeout  time.Duration
	path              string
	params            url.Values
	headers           http.Header
	cookie            *http.Cookie
	body              io.Reader
	rawBody           []byte
	baseAuth          bool
	baseAuthUsername  string
	baseAuthPassword  string
	gzip              bool
	retry             int
	retryInterval     time.Duration
	retryHttpStatuses []int // 重试的http状态码
}

type AdvanceResponse struct {
	Body       []byte
	Header     http.Header
	StatusCode int
	Status     string
	Time       int64
}

func NewAdvanceSettings(rwTimeout time.Duration, retry int, retryInterval time.Duration, retryHttpStatuses ...int) *AdvanceSettings {
	return &AdvanceSettings{
		readWriteTimeout:  rwTimeout,
		params:            url.Values{},
		headers:           http.Header{},
		baseAuth:          false,
		gzip:              false,
		retry:             retry,
		retryInterval:     retryInterval,
		retryHttpStatuses: retryHttpStatuses,
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

func (setting *AdvanceSettings) SetRetryHttpStatuses(retryHttpStatuses []int) *AdvanceSettings {
	setting.retryHttpStatuses = retryHttpStatuses
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

func (setting *AdvanceSettings) retryCheck(responseStatusCode int) bool {
	for _, code := range setting.retryHttpStatuses {
		if code == responseStatusCode {
			return true
		}
	}

	return false
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

func (client *AdvanceHttpClient) ToCurlCommand(method, targetUrl string, setting *AdvanceSettings) (string, error) {
	req, err := genHttpRequest(method, targetUrl, setting)
	if err != nil {
		return "", err
	}

	cmd, err := http2curl.GetCurlCommand(req)
	if err != nil {
		return "", err
	}

	return cmd.String(), nil
}

func genHttpRequest(method, url string, setting *AdvanceSettings) (*http.Request, error) {
	for _, code := range setting.retryHttpStatuses {
		if code <= 201 {
			return nil, fmt.Errorf("设置的重试http状态码包含201及以下, %+v", setting.retryHttpStatuses)
		}

		if code >= 500 {
			return nil, fmt.Errorf("设置的重试http状态码包含500及以上服务端错误的状态码, %+v", setting.retryHttpStatuses)
		}
	}

	setting.body = bytes.NewBuffer(setting.rawBody)

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

	if setting.retry <= 0 {
		setting.retry = 1
	} else {
		setting.retry++
	}

	adresp := &AdvanceResponse{}

	startTime := time.Now()
	for i := 0; i < setting.retry; i++ {
		// solve Golang http post error : http: ContentLength=355 with Body length 0 bug
		err := client.doOnce(method, url, setting, adresp)
		if err != nil {
			// 达到retry的次数
			if i == setting.retry-1 {
				return nil, err
			}
			time.Sleep(setting.retryInterval)
			continue
		}

		if setting.retryCheck(adresp.StatusCode) {
			if i == setting.retry-1 {
				break
			}

			time.Sleep(setting.retryInterval)
			continue
		}

		break
	}

	adresp.Time = int64(time.Now().Sub(startTime))

	return adresp, nil
}

func (client *AdvanceHttpClient) doOnce(method, url string, setting *AdvanceSettings, adresp *AdvanceResponse) error {
	req, err := genHttpRequest(method, url, setting)
	if err != nil {
		return err
	}

	if setting.readWriteTimeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), setting.readWriteTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := client.client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	adresp.Header = resp.Header
	adresp.StatusCode = resp.StatusCode
	adresp.Status = resp.Status

	if setting.gzip && resp.Header.Get("Content-Encoding") == "gzip" {
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
