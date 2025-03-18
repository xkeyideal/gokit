package httpkit

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/moul/http2curl"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
)

type HttpClient struct {
	c                 *http.Client
	rwTimeout         time.Duration
	params            url.Values
	headers           http.Header
	cookie            *http.Cookie
	body              io.Reader
	rawBody           []byte //原始body备份使用，retry的时候使用
	baseAuth          bool
	baseAuthUsername  string
	baseAuthPassword  string
	gzip              bool
	retry             int
	retryInterval     time.Duration
	retryHttpStatuses []int // 重试的http状态码
	otelHttp          bool  // 支持opentelemetry otelhttptrace context inject

	ctx    context.Context
	cancel context.CancelFunc
}

func NewHttpClient(rwTimeout time.Duration, retry int,
	retryInterval, connTimeout time.Duration, tlsCfg *tls.Config,
	retryHttpStatuses ...int,
) *HttpClient {
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
		c:                 client,
		params:            url.Values{},
		headers:           http.Header{},
		rwTimeout:         rwTimeout,
		baseAuth:          false,
		gzip:              false,
		retry:             retry,
		retryInterval:     retryInterval,
		retryHttpStatuses: retryHttpStatuses,
	}
}

func NewHttpClientWithTransport(rwTimeout time.Duration, retry int,
	retryInterval, connTimeout time.Duration, tlsCfg *tls.Config,
	transport http.RoundTripper,
	retryHttpStatuses ...int,
) *HttpClient {
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

	return &HttpClient{
		c:                 client,
		params:            url.Values{},
		headers:           http.Header{},
		rwTimeout:         rwTimeout,
		baseAuth:          false,
		gzip:              false,
		retry:             retry,
		retryInterval:     retryInterval,
		retryHttpStatuses: retryHttpStatuses,
	}
}

func (client *HttpClient) EnableGZip(gzip bool) *HttpClient {
	client.gzip = gzip
	return client
}

func (client *HttpClient) EnableOtelHttp(otelHttp bool) *HttpClient {
	client.otelHttp = otelHttp
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

func (client *HttpClient) SetRetryHttpStatuses(retryHttpStatuses []int) *HttpClient {
	client.retryHttpStatuses = retryHttpStatuses
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
	rawBody, _ := io.ReadAll(body)
	client.body = bytes.NewBuffer(rawBody)
	client.rawBody = rawBody
	return client
}

func (client *HttpClient) Get(targetUrl string) (*AdvanceResponse, error) {
	client.body = nil
	client.rawBody = []byte{}
	return client.do(context.Background(), "GET", targetUrl)
}

func (client *HttpClient) Post(targetUrl string) (*AdvanceResponse, error) {
	return client.do(context.Background(), "POST", targetUrl)
}

func (client *HttpClient) Put(targetUrl string) (*AdvanceResponse, error) {
	return client.do(context.Background(), "PUT", targetUrl)
}

func (client *HttpClient) Delete(targetUrl string) (*AdvanceResponse, error) {
	return client.do(context.Background(), "DELETE", targetUrl)
}

func (client *HttpClient) Head(targetUrl string) (*AdvanceResponse, error) {
	return client.do(context.Background(), "HEAD", targetUrl)
}

func (client *HttpClient) Do(method, targetUrl string) (*AdvanceResponse, error) {
	return client.do(context.Background(), method, targetUrl)
}

func (client *HttpClient) GetWithContext(ctx context.Context, targetUrl string) (*AdvanceResponse, error) {
	client.body = nil
	client.rawBody = []byte{}
	return client.do(ctx, "GET", targetUrl)
}

func (client *HttpClient) PostWithContext(ctx context.Context, targetUrl string) (*AdvanceResponse, error) {
	return client.do(ctx, "POST", targetUrl)
}

func (client *HttpClient) PutWithContext(ctx context.Context, targetUrl string) (*AdvanceResponse, error) {
	return client.do(ctx, "PUT", targetUrl)
}

func (client *HttpClient) DeleteWithContext(ctx context.Context, targetUrl string) (*AdvanceResponse, error) {
	return client.do(ctx, "DELETE", targetUrl)
}

func (client *HttpClient) HeadWithContext(ctx context.Context, targetUrl string) (*AdvanceResponse, error) {
	return client.do(ctx, "HEAD", targetUrl)
}

func (client *HttpClient) DoWithContext(ctx context.Context, method, targetUrl string) (*AdvanceResponse, error) {
	return client.do(ctx, method, targetUrl)
}

func (client *HttpClient) genHttpRequest(ctx context.Context, method, targetUrl string) (*http.Request, error) {
	for _, code := range client.retryHttpStatuses {
		if code <= 201 {
			return nil, fmt.Errorf("设置的重试http状态码包含201及以下, %+v", client.retryHttpStatuses)
		}

		if code >= 500 {
			return nil, fmt.Errorf("设置的重试http状态码包含500及以上服务端错误的状态码, %+v", client.retryHttpStatuses)
		}
	}

	u, err := url.Parse(targetUrl)
	if err != nil {
		return nil, err
	}

	if u.RawQuery != "" {
		return nil, fmt.Errorf("url中不能存在query参数[%s]，请使用client.SetParam等方法预设置", u.RawQuery)
	}

	u.RawQuery = client.params.Encode()
	client.body = bytes.NewBuffer(client.rawBody)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), client.body)
	if err != nil {
		return nil, err
	}

	for key, values := range client.headers {
		if strings.ToLower(key) == "host" {
			if len(values) > 0 {
				req.Host = values[0]
			}
		} else {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}

	if client.cookie != nil {
		req.Header.Add("Cookie", client.cookie.String())
	}

	if client.baseAuth {
		req.SetBasicAuth(client.baseAuthUsername, client.baseAuthPassword)
	}

	if client.otelHttp {
		ctx = httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
		req = req.WithContext(ctx)
		otelhttptrace.Inject(ctx, req)
	}

	return req, nil
}

func (client *HttpClient) ToCurlCommand(ctx context.Context, method, targetUrl string) (string, error) {
	req, err := client.genHttpRequest(ctx, method, targetUrl)
	if err != nil {
		return "", err
	}

	cmd, err := http2curl.GetCurlCommand(req)
	if err != nil {
		return "", err
	}

	return cmd.String(), nil
}

func (client *HttpClient) do(ctx context.Context, method, targetUrl string) (*AdvanceResponse, error) {
	if client.retry <= 0 {
		client.retry = 1
	} else {
		client.retry++
	}

	adresp := &AdvanceResponse{}

	startTime := time.Now()
	for i := 0; i < client.retry; i++ {
		err := client.doOnce(ctx, method, targetUrl, adresp)
		if err != nil {
			// 达到retry的次数
			if i == client.retry-1 {
				return nil, err
			}
			time.Sleep(client.retryInterval)
			continue
		}

		if client.retryCheck(adresp.StatusCode) {
			if i == client.retry-1 {
				break
			}

			time.Sleep(client.retryInterval)
			continue
		}

		break
	}

	adresp.Time = int64(time.Now().Sub(startTime))

	return adresp, nil
}

func (client *HttpClient) retryCheck(responseStatusCode int) bool {
	for _, code := range client.retryHttpStatuses {
		if code == responseStatusCode {
			return true
		}
	}

	return false
}

func (client *HttpClient) doOnce(ctx context.Context, method, targetUrl string, adresp *AdvanceResponse) error {
	req, err := client.genHttpRequest(ctx, method, targetUrl)
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
		return err
	}

	defer resp.Body.Close()

	adresp.Header = resp.Header
	adresp.StatusCode = resp.StatusCode
	adresp.Status = resp.Status

	if client.gzip && resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			return err
		}

		adresp.Body = body
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	adresp.Body = body
	return nil
}
