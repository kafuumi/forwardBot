package forwardBot

import (
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/qbot"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Sink interface {
	Receive(msg *push.Msg) error
}

type PushSink struct {
	pusher push.Pusher
}

func NewPushSink(p push.Pusher) *PushSink {
	return &PushSink{pusher: p}
}

func (p *PushSink) Receive(msg *push.Msg) error {
	logger.Info("PushSink 推送消息")
	err := p.pusher.PushMsg(msg)
	if err != nil {
		return errors.Wrap(err, "PushSink推送消息失败")
	} else {
		logger.Info("PushSink 推送消息成功")
	}
	return nil
}

const (
	CQBotCmdHelp             = "/啵啵"
	CQBotCmdAll              = "/订阅全部"
	CQBotCmdAllCancel        = "/取消订阅"
	CQBotCmdBiliLive         = "/b站开播"
	CQBotCmdBiliLiveCancel   = "/取消b站开播"
	CQBotCmdBiliDyn          = "/b站动态"
	CQBotCmdBiliDynCancel    = "/取消b站动态"
	CQBotCmdTiktokLive       = "/抖音开播"
	CQBotCmdTiktokLiveCancel = "/取消抖音开播"
	CQBotCmdPushTest         = "/推送测试"
)
const AllMsyNum = 3

type CQBotSink struct {
	bot       *qbot.CQBot
	table     map[uint64][]uint64 //接收推送消息的频道
	bufSize   int
	lock      sync.RWMutex
	heartbeat int64 //上一次收到心跳包的时间
}

func NewCQBotSink(host, token string, bufSize int) *CQBotSink {
	logger.WithFields(logrus.Fields{
		"host":    host,
		"token":   token,
		"bufSize": bufSize,
	}).Info("创建CQBot")
	qbot.SetHandler(qbot.EchoSendGuildMsg, func(msg *qbot.EchoMsg) bool {
		if msg.RetCode == 2 {
			logger.WithFields(logrus.Fields{
				"code":    msg.RetCode,
				"status":  msg.Status,
				"wording": msg.Wording,
			}).Warn("频道发送消息失败")
		} else {
			logger.WithFields(logrus.Fields{
				"code":   msg.RetCode,
				"status": msg.Status,
			}).Info("频道中发送消息成功")
		}
		return true
	})
	return &CQBotSink{
		bot:     qbot.NewCQBot(host, token),
		table:   make(map[uint64][]uint64),
		bufSize: bufSize,
	}
}

func (c *CQBotSink) Receive(msg *push.Msg) error {
	logger.Info("CQBot发送消息")
	if len(c.table) == 0 {
		logger.Info("无频道订阅消息")
		return nil
	}
	text := strings.Builder{}
	text.WriteString(msg.Times.Format("2006-01-02 15:04"))
	text.WriteByte('\n')
	text.WriteString(fmt.Sprintf("%s %s\n", msg.Author, msg.Title))
	text.WriteString(msg.Text)
	if msg.Src != "" {
		text.WriteByte('\n')
		text.WriteString(msg.Src)
	}
	for i := range msg.Img {
		img := &qbot.CQCode{
			Types: "image",
			Data: map[string]string{
				"file": msg.Img[i],
			},
		}
		text.WriteString(img.String())
	}
	msgContent := text.String()
	var err error
	c.lock.RLock()
	defer c.lock.RUnlock()
	for gId, cIds := range c.table {
		index := msg.Flag
		if index >= len(cIds) {
			logger.WithFields(logrus.Fields{
				"index":     index,
				"len(cIds)": len(cIds),
			}).Warn("未知错误，不应该发生的情况")
			continue
		}
		cId := cIds[index]
		if cId == 0 {
			logger.WithFields(logrus.Fields{
				"guildId":   gId,
				"channelId": cId,
				"flag":      msg.Flag,
			}).Debug("当前频道未订阅该消息")
			continue
		}
		err = c.bot.SendGuildMsg(gId, cId, msgContent)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"guildId":   gId,
				"channelId": cId,
				"err":       err,
			}).Error("发送频道消息失败")
		}
	}
	return nil
}

func (c *CQBotSink) Listen(ctx context.Context) error {
	logger.Info("CQBot监听消息")
	err := c.bot.Connect(ctx)
	if err != nil {
		return err
	}
	ch := make(chan *qbot.CQBotMsg, c.bufSize)
	go c.bot.ListenMsg(ctx, ch)
	//心跳包检测
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				logger.Debug("CQBot停止心跳包检测")
				return
			case now := <-ticker.C:
				if now.Unix()-c.heartbeat > 20 {
					logger.Warn("CQBot超过20秒未收到心跳包")
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			c.bot.DisConnect()
			logger.Info("CQBot停止")
			return nil
		case msg := <-ch:
			if msg.MsgType == qbot.CQBotHeartMsg {
				c.heartbeat = msg.Times
				logger.Debug("CQBot收到心跳包")
				continue
			}
			if msg.MsgType != qbot.CQBotGuildMsg {
				logger.Info("CQBot收到非频道消息")
				continue
			}
			cmd := qbot.ParseCQBotCmd(msg.Text)
			//不是指令
			if cmd == nil {
				logger.WithField("text", msg.Text).Debug("解析指令失败")
				continue
			}
			gId, cId := msg.SourceId, msg.SubSourceId
			logger.WithFields(logrus.Fields{
				"guildId":   gId,
				"channelId": cId,
				"cmd":       cmd.Cmd,
				"params":    cmd.Params,
			}).Info("接收到指令")
			switch cmd.Cmd {
			case CQBotCmdHelp:
				at := &qbot.CQCode{
					Types: "at",
					Data: map[string]string{
						"qq": strconv.FormatUint(msg.SenderId, 10),
					},
				}
				content := fmt.Sprintf("%s当前可用指令：\n"+
					"%s 订阅所有消息\n"+
					"%s 取消消息订阅\n"+
					"%s 订阅b站开播消息\n"+
					"%s\n"+
					"%s 订阅b站动态更新消息\n"+
					"%s\n"+
					"%s 订阅抖音开播消息\n"+
					"%s\n"+
					"%s", at.String(), CQBotCmdAll, CQBotCmdAllCancel,
					CQBotCmdBiliLive, CQBotCmdBiliLiveCancel,
					CQBotCmdBiliDyn, CQBotCmdBiliDynCancel,
					CQBotCmdTiktokLive, CQBotCmdTiktokLiveCancel,
					CQBotCmdPushTest)
				_ = c.bot.SendGuildMsg(gId, cId, content)
			case CQBotCmdAll:
				c.SubscribeAll(gId, cId)
			case CQBotCmdAllCancel:
				c.UnsubscribeAll(gId, cId)
			case CQBotCmdBiliDyn:
				c.Subscribe(gId, cId, BiliDynMsg)
			case CQBotCmdBiliLive:
				c.Subscribe(gId, cId, BiliLiveMsg)
			case CQBotCmdTiktokLive:
				c.Subscribe(gId, cId, TikTokLiveMsg)
			case CQBotCmdTiktokLiveCancel:
				c.Unsubscribe(gId, cId, TikTokLiveMsg)
			case CQBotCmdBiliLiveCancel:
				c.Unsubscribe(gId, cId, BiliLiveMsg)
			case CQBotCmdBiliDynCancel:
				c.Unsubscribe(gId, cId, BiliDynMsg)
			default:
				logger.WithField("cmd", cmd.Cmd).Info("不支持的指令")
			}
		}
	}
}

func (c *CQBotSink) SubscribeAll(gId, cId uint64) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cIds := c.table[gId]
	var err error
	if len(cIds) == 0 {
		cIds = make([]uint64, AllMsyNum)
	}
	//当前频道是否已经订阅
	ok := true
	for i := range cIds {
		if cIds[i] != cId {
			cIds[i] = cId
			ok = false
		}
	}
	c.table[gId] = cIds
	if ok {
		err = c.bot.SendGuildMsg(gId, cId, "当前频道已经订阅消息")
	} else {
		err = c.bot.SendGuildMsg(gId, cId, "订阅成功")
	}
	if err != nil {
		logger.WithFields(logrus.Fields{
			"guildId":   gId,
			"channelId": cId,
			"err":       err,
		}).Error("发送频道消息失败")
	}
}

func (c *CQBotSink) UnsubscribeAll(gId, cId uint64) {
	c.lock.Lock()
	defer c.lock.Unlock()
	var err error
	if _, ok := c.table[gId]; ok {
		delete(c.table, gId)
		err = c.bot.SendGuildMsg(gId, cId, "取消成功")
	} else {
		err = c.bot.SendGuildMsg(gId, cId, "当前频道未订阅消息")
	}
	if err != nil {
		logger.WithFields(logrus.Fields{
			"guildId":   gId,
			"channelId": cId,
			"err":       err,
		}).Error("发送频道消息失败")
	}
}

func (c *CQBotSink) Subscribe(gId, cId uint64, mask int) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cIds := c.table[gId]
	if len(cIds) == 0 {
		cIds = make([]uint64, AllMsyNum)
	}
	var err error
	if cIds[mask] == cId {
		err = c.bot.SendGuildMsg(gId, cId, "当前频道已经设置订阅")
	} else {
		cIds[mask] = cId
		c.table[gId] = cIds
		err = c.bot.SendGuildMsg(gId, cId, "订阅成功")
	}
	if err != nil {
		logger.WithFields(logrus.Fields{
			"guildId":   gId,
			"channelId": cId,
			"err":       err,
		}).Error("发送频道消息失败")
	}
}

func (c *CQBotSink) Unsubscribe(gId, cId uint64, mask int) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cIds := c.table[gId]
	var err error
	if len(cIds) != 0 && cIds[mask] == cId {
		cIds[mask] = 0
		c.table[gId] = cIds
		err = c.bot.SendGuildMsg(gId, cId, "取消成功")
	} else {
		err = c.bot.SendGuildMsg(gId, cId, "当前频道未订阅消息")
	}
	if err != nil {
		logger.WithFields(logrus.Fields{
			"guildId":   gId,
			"channelId": cId,
			"err":       err,
		}).Error("发送频道消息失败")
	}
	if len(cIds) != 0 {
		num := uint64(0)
		for i := range cIds {
			num |= cIds[i]
		}
		if num == 0 {
			delete(c.table, gId)
		}
	}
}
