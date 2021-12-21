// Package http 封装 http 接口调用
package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"goreplay/config"
)

const (
	timeout = 5 * time.Second

	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
)

// Client http client
//go:generate mockgen -destination=mock/client_mock.go -package=mock -source client.go
type Client interface {
	// Get send http request by get mehod
	Get(url string, rspbody interface{}, opts ...Option) error
	// Post send http request by post mehod
	Post(url string, reqbody interface{}, rspbody interface{}, opts ...Option) error
}

// client http client
type client struct {
	cli *http.Client
}

// NewClient new http client
func NewClient() Client {
	return &client{
		cli: &http.Client{
			Timeout: timeout,
		},
	}
}

// Get send http request by get mehod
func (c *client) Get(rawurl string, rspbody interface{}, opts ...Option) error {
	return c.send(rawurl, http.MethodGet, nil, rspbody, opts...)
}

// Post send http request by post mehod
func (c *client) Post(rawurl string, reqbody interface{}, rspbody interface{}, opts ...Option) error {
	return c.send(rawurl, http.MethodPost, reqbody, rspbody, opts...)
}

func (c *client) send(rawurl, method string, reqbody interface{}, rspbody interface{}, opts ...Option) error {
	var body io.Reader
	if reqbody != nil {
		buf, err := json.Marshal(reqbody)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(buf)
	}
	req, err := http.NewRequest(method, rawurl, body)
	if err != nil {
		return err
	}
	req.Header.Add(contentTypeHeader, jsonContentType)

	config := &config.Settings.OutputLogReplayConfig
	req.Header.Add("AppId", config.APPID)
	req.Header.Add("AppKey", config.APPKey)
	req.Header.Add("EPP-Gateway-Env", config.Env)
	req.Header.Add("Rewrite-Request", "true")

	rsp, err := c.cli.Do(req)
	if err != nil {
		return err
	}
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status code: %v", rsp.StatusCode)
	}

	defer rsp.Body.Close()
	buf, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("read response body error: %v", err)
	}
	return json.Unmarshal(buf, rspbody)
}
