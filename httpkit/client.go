package httpkit

import (
	"compress/gzip"
	"context"
	"crypto/tls"
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
	baseAuth         bool
	baseAuthUsername string
	baseAuthPassword string
	gzip             bool
	retry            int
	retryInterval    time.Duration
}

func NewHttpClient(rwTimeout time.Duration, retry int, retryInterval, connTimeout time.Duration, tlsCfg *tls.Config) *HttpClient {
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
	client.body = body
	return client
}

func (client *HttpClient) Get(targetUrl string) (*AdvanceResponse, error) {
	client.body = nil
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

func (client *HttpClient) do(method, targetUrl string) (*AdvanceResponse, error) {
	u, err := url.Parse(targetUrl)
	if err != nil {
		return nil, err
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

	if client.rwTimeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), client.rwTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	var resp *http.Response
	var err2 error

	startTime := time.Now()
	if client.retry > 0 {
		for i := 0; i < client.retry; i++ {
			resp, err2 = client.c.Do(req)
			if err2 != nil {
				time.Sleep(client.retryInterval)
				continue
			}

			// 非2xx 或 3xx的状态码也认为是服务端响应出错，需重试
			if !(resp.StatusCode >= 200 && resp.StatusCode < 400) {
				time.Sleep(client.retryInterval)
				continue
			}

			break
		}
	} else {
		resp, err2 = client.c.Do(req)
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

	if client.gzip && resp.Header.Get("Content-Encoding") == "gzip" {
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
