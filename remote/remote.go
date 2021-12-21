package remote

import (
	"fmt"

	"goreplay/client/http"
	"goreplay/config"
)

var httpCli = http.NewClient()

// Send 调用 logreplay 后台服务接口
// uri 不包含 host 部分
func Send(uri string, req, rsp interface{}) error {
	return send(uri, req, rsp)
}

// send 调用 logreplay 后台服务接口
func send(uri string, req, rsp interface{}) error {
	conf := &config.Settings.OutputLogReplayConfig
	url := fmt.Sprintf("http://%s%s", conf.GatewayHost(), uri)
	return httpCli.Post(url, req, rsp)
}
