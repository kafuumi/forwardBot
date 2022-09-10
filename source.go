package forwardBot

import (
	"context"
	"fmt"
	"forwardBot/push"
	"github.com/sirupsen/logrus"
	"time"
)

type Source interface {
	// Send 向ch中发送信息
	Send(ctx context.Context, ch chan<- *push.Msg)
}

var testSource = &CustomSource{ch: make(chan *push.Msg, 10)}

var _ Source = (*CustomSource)(nil)

// CustomSource 自定义source，用于测试推送系统
type CustomSource struct {
	ch      chan *push.Msg
	running bool
}

func (c *CustomSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	c.running = true
	for {
		select {
		case <-ctx.Done():
			c.running = false
			return
		case msg := <-c.ch:
			logger.WithFields(logrus.Fields{
				"title": msg.Title,
				"text":  msg.Text,
			}).Info("CustomSource 发送消息")
			ch <- msg
		}
	}
}

func (c *CustomSource) Test(flags int) {
	logger.Info("[CustomSource]产生测试消息")
	if !c.running {
		logger.Warn("[CustomSource]并未运行")
	}
	go func() {
		if flags >= AllMsgNum {
			logger.WithFields(logrus.Fields{
				"flags":  flags,
				"update": 0,
			}).Warn("[CustomSource]错误的消息类型，修正为0")
			flags = 0
		}
		msg := &push.Msg{
			Times:  time.Now(),
			Flag:   flags,
			Author: "Bot",
			Title:  "推送测试",
			Text:   fmt.Sprintf("测试消息，flag=%d", flags),
			Img:    []string{"https://i0.hdslb.com/bfs/emote/332a6df0e6def8da77e09310a62f3bffdc397640.png"},
		}
		c.ch <- msg
	}()
}
