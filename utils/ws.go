package utils

import (
	"net/http"
	"time"

	"github.com/liu2hai/chive/logs"

	"github.com/gorilla/websocket"
)

func Reconnect(wsurl string, ex string, tag string) *websocket.Conn {
	var c *websocket.Conn = nil
	for {
		c = WSConnect(wsurl, ex, tag)
		if c == nil {
			logs.Error("[%s %s]连接失败，暂停5s重连...", ex, tag)
			<-time.After(5 * time.Second)
		} else {
			break
		}
	}
	return c
}

func WSConnect(wsurl string, ex string, tag string) *websocket.Conn {
	req, err := http.NewRequest("GET", wsurl, nil)
	if err != nil {
		logs.Error("[%s %s]发起websocket连接请求，构建request出错", ex, tag)
		return nil
	}

	c, httpresp, err := websocket.DefaultDialer.Dial(wsurl, req.Header)
	if err != nil {
		logs.Error("[%s %s]发起websocket连接请求失败，返回错误:%s", ex, tag, err.Error())
		return nil
	}

	if httpresp.StatusCode != http.StatusSwitchingProtocols {
		logs.Error("[%s %s]发起websocket连接请求失败，返回报状态码:%d", ex, tag, httpresp.StatusCode)
		c.Close()
		return nil
	}
	return c
}
