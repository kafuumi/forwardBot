package req

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/pkg/errors"
)

var (
	comHeader = D{
		{"Accept-Encoding", "gzip, deflate, br"},
		{"Accept-Language", "zh-CN,zh;q=0.9"},
		{"cache-control", "no-cache"},
		{"pragma", "no-cache"},
		{"sec-fetch-mod", "navigate"},
		{"sec-fetch-site", "same-site"},
		{"sec-fetch-user", "?1"},
		{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36"},
	}
	defaultClient = New(5)
)

type C struct {
	client  *http.Client
	cookies map[string]string
}

func New(timeout int) *C {
	return &C{
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

func (c *C) SetCookies(name, value string) {
	if c.cookies == nil {
		c.cookies = make(map[string]string)
	}
	c.cookies[name] = value
}

// 选择对应的解压算法解压响应体
func compress(resp *http.Response) (reader io.Reader) {
	contentEncoding := resp.Header.Get("Content-Encoding")
	switch contentEncoding {
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "gzip":
		reader, _ = gzip.NewReader(resp.Body)
	case "deflate":
		reader = flate.NewReader(resp.Body)
	default:
		reader = resp.Body
	}
	return
}

// 发送请求，method 为请求方法，link为请求地址，params为url参数，body为请求体，headers为请求头
func (c *C) request(method, link string, params D, body io.Reader,
	headers ...E) (buf *bytes.Buffer, err error) {
	value := make(url.Values)
	for i := range params {
		value.Add(params[i].Name, fmt.Sprint(params[i].Value))
	}
	if len(value) != 0 {
		link = link + "?" + value.Encode()
	}

	req, err := http.NewRequest(method, link, body)
	if err != nil {
		return nil, errors.Wrap(err, "create request error")
	}
	for i := range comHeader {
		req.Header.Set(comHeader[i].Name, comHeader[i].Value.(string))
	}
	for i := range headers {
		req.Header.Set(headers[i].Name, fmt.Sprint(headers[i].Value))
	}
	for k := range c.cookies {
		req.AddCookie(&http.Cookie{
			Name:  k,
			Value: c.cookies[k],
			Path:  "/",
		})
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("request fail, status=%s", resp.Status))
	}

	reader := compress(resp)
	buf = new(bytes.Buffer)
	_, err = io.Copy(buf, reader)
	if err != nil {
		err = errors.Wrap(err, "read body error")
	}
	return
}

func (c *C) Post(link string, params D, body io.Reader,
	headers ...E) (buf *bytes.Buffer, err error) {
	return c.request(http.MethodPost, link, params, body, headers...)
}

func (c *C) Get(link string, params D, body io.Reader,
	headers ...E) (buf *bytes.Buffer, err error) {
	return c.request(http.MethodGet, link, params, body, headers...)
}

func Get(link string, params D, headers ...E) (buf *bytes.Buffer, err error) {
	return defaultClient.Get(link, params, nil, headers...)
}

func Post(link string, params D, body io.Reader, headers ...E) (buf *bytes.Buffer, err error) {
	return defaultClient.Post(link, params, body, headers...)
}
