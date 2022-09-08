package qbot

import (
	"context"
	"fmt"
	"forwardBot/req"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	apiSendGuildMsg = "send_guild_channel_msg" //在频道中发送消息
)

const (
	EchoSendGuildMsg = "SendGuildChannelMsgEcho"
)

// CQBot 使用go-CQHttp实现的机器人
type CQBot struct {
	host  string
	token string
	conn  *websocket.Conn
}

func NewCQBot(host, token string) *CQBot {
	return &CQBot{
		host:  host,
		token: token,
	}
}

// Connect 与服务端建立连接
func (c *CQBot) Connect(ctx context.Context) error {
	dialer := new(websocket.Dialer)
	var host string
	if c.token != "" {
		host = fmt.Sprintf("%s?access_token=%s", c.host, c.token)
	} else {
		host = c.host
	}
	conn, _, err := dialer.DialContext(ctx, host, nil)
	if err != nil {
		return errors.Wrap(err, "connect server fail")
	}
	c.conn = conn
	return nil
}

// DisConnect 断开链接
func (c *CQBot) DisConnect() {
	_ = c.conn.Close()
}

// ListenMsg 监听服务端推送的消息，解析消息体并发送到通道中
func (c *CQBot) ListenMsg(ctx context.Context, ch chan<- *CQBotMsg) {
	var msgType int
	var data []byte
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgType, data, err = c.conn.ReadMessage()
			if err != nil {

			}
			if msgType != websocket.TextMessage {

			}
			msg := parseCQBotMsg(data)
			if msg == nil {

			} else {
				ch <- msg
			}
		}
	}
}

func (c *CQBot) SendGuildMsg(guildId, channelId uint64, msg string) error {
	body := req.D{
		{"action", apiSendGuildMsg},
		{"echo", EchoSendGuildMsg},
		{"params", req.D{
			{"guild_id", guildId},
			{"channel_id", channelId},
			{"message", msg},
		}},
	}
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(body.Json()))
	return err
}
