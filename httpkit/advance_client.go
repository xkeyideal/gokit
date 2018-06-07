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

	client := &http.Client{Transport: tr}

	return &AdvanceHttpClient{
		scheme: scheme,
		host:   host,
		client: client,
	}
}

type AdvanceSettings struct {
	readWriteTimeout int
	path             string
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

type AdvanceResponse struct {
	Body       []byte
	Header     http.Header
	StatusCode int
	Status     string
	Time       int
}

func NewAdvanceSettings(rwTimeout, retry int, retryInterval time.Duration) *AdvanceSettings {
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
	setting.body = body
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

func (client *AdvanceHttpClient) do(method, uri string, setting *AdvanceSettings) (*AdvanceResponse, error) {
	url := setting.urlString(client.scheme, client.host, uri)

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

	if setting.readWriteTimeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), time.Duration(setting.readWriteTimeout)*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
	}

	var resp *http.Response
	var err2 error

	startTime := time.Now().Nanosecond()
	if setting.retry > 0 {
		for i := 0; i < setting.retry; i++ {
			resp, err2 = client.client.Do(req)
			if err2 != nil {
				time.Sleep(setting.retryInterval)
				continue
			}

			break
		}
	} else {
		resp, err2 = client.client.Do(req)
	}
	if err2 != nil {
		return nil, err2
	}

	endTime := time.Now().Nanosecond()

	defer resp.Body.Close()

	adresp := &AdvanceResponse{
		Header:     resp.Header,
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Time:       endTime - startTime,
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

		return adresp, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	adresp.Body = body

	return adresp, nil
}
