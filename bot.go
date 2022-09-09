package forwardBot

import (
	"context"
	"forwardBot/push"
	"github.com/sirupsen/logrus"
)

const (
	BiliLiveMsg = iota
	BiliDynMsg
	TikTokLiveMsg
)

type Bot struct {
	sources []Source
	sinks   []Sink
	ch      chan *push.Msg
}

func NewBot(buf int) *Bot {
	logger.WithFields(logrus.Fields{
		"buf": buf,
	}).Info("创建 bot")
	return &Bot{
		ch: make(chan *push.Msg, buf),
	}
}

func (b *Bot) AppendSource(s ...Source) {
	for _, source := range s {
		if source != nil {
			b.sources = append(b.sources, source)
		} else {
			logger.Warn("添加的Source为nil")
		}
	}
	logger.WithFields(logrus.Fields{
		"len(b.sources)": len(b.sources),
		"len(sources)":   len(s),
	}).Debug("添加Source")
}

func (b *Bot) AppendSink(s ...Sink) {
	for _, sink := range s {
		if sink != nil {
			b.sinks = append(b.sinks, sink)
		} else {
			logger.Warn("添加的Sink为nil")
		}
	}
	logger.WithFields(logrus.Fields{
		"len(b.sinks)": len(b.sinks),
		"len(sinks)":   len(s),
	}).Debug("添加Sink")
}

func (b *Bot) Run(ctx context.Context) {
	logger.Info("启动bot")
	for _, s := range b.sources {
		go s.Send(ctx, b.ch)
	}
	for {
		select {
		case <-ctx.Done():
			logger.Info("bot退出")
			return
		case msg := <-b.ch:
			logger.WithFields(logrus.Fields{
				"author":   msg.Author,
				"title":    msg.Title,
				"src":      msg.Src,
				"len(img)": len(msg.Img),
				"flag":     msg.Flag,
			}).Info("接收到msg")
			for _, s := range b.sinks {
				go func(s Sink) {
					err := s.Receive(msg)
					if err != nil {
						logger.WithField("error", err).Error("bot发送消息失败")
					}
				}(s)
			}
		}
	}
}
