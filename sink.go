package forwardBot

import (
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/qbot"
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
	return p.pusher.PushMsg(msg)
}

const (
	CQBotCmdHelp             = "/啵啵"
	CQBotCmdBiliLive         = "/b站开播"
	CQBotCmdBiliDyn          = "/b站动态"
	CQBotCmdBiliLiveCancel   = "/取消b站开播订阅"
	CQBotCmdBiliDynCancel    = "/取消b站动态订阅"
	CQBotCmdTiktokLive       = "/抖音开播"
	CQBotCmdTiktokLiveCancel = "/取消抖音开播订阅"
)

type CQBotSink struct {
	bot       *qbot.CQBot
	table     map[uint64]uint64 //接收推送消息的频道
	bufSize   int
	flag      map[uint64]int
	lock      sync.RWMutex
	heartbeat int64 //上一次收到心跳包的时间
}

func NewCQBotSink(host, token string, bufSize int) *CQBotSink {
	qbot.SetHandler(qbot.EchoSendGuildMsg, func(msg *qbot.EchoMsg) bool {
		if msg.RetCode == 2 {
			fmt.Println(msg.Wording)
		}
		return true
	})
	return &CQBotSink{
		bot:     qbot.NewCQBot(host, token),
		table:   make(map[uint64]uint64),
		bufSize: bufSize,
		flag:    make(map[uint64]int),
	}
}

func (c *CQBotSink) Receive(msg *push.Msg) error {
	if len(c.table) == 0 {
		return nil
	}
	text := strings.Builder{}
	text.WriteString(msg.Times.Format("2006-01-02 15:04"))
	text.WriteByte('\n')
	text.WriteString(fmt.Sprintf("%s %s\n", msg.Author, msg.Title))
	text.WriteString(msg.Text)
	text.WriteByte('\n')
	if msg.Src != "" {
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
	for gId, cId := range c.table {
		cFlag, flag := c.flag[gId], msg.Flag
		if cFlag&flag == 0 {
			continue
		}
		err = c.bot.SendGuildMsg(gId, cId, msgContent)
		if err != nil {
			//TODO
			fmt.Println(err)
		}
	}
	return nil
}

func (c *CQBotSink) Listen(ctx context.Context) error {
	err := c.bot.Connect(ctx)
	if err != nil {
		return err
	}
	ch := make(chan *qbot.CQBotMsg, c.bufSize)
	go c.bot.ListenMsg(ctx, ch)
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				if now.Unix()-c.heartbeat > 20 {
					fmt.Println("heart error")
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			c.bot.DisConnect()
			return nil
		case msg := <-ch:
			if msg.MsgType == qbot.CQBotHeartMsg {
				c.heartbeat = msg.Times
				continue
			}
			if msg.MsgType != qbot.CQBotGuildMsg {
				continue
			}
			cmd := qbot.ParseCQBotCmd(msg.Text)
			//不是指令
			if cmd == nil {
				continue
			}
			gId, cId := msg.SourceId, msg.SubSourceId
			switch cmd.Cmd {
			case CQBotCmdHelp:
				at := &qbot.CQCode{
					Types: "at",
					Data: map[string]string{
						"qq": strconv.FormatUint(msg.SenderId, 10),
					},
				}
				_ = c.bot.SendGuildMsg(gId, cId,
					fmt.Sprintf("%s当前可用指令：\n%s\n%s\n%s\n%s\n%s\n%s",
						at.String(), CQBotCmdBiliDyn, CQBotCmdBiliLive,
						CQBotCmdBiliDynCancel, CQBotCmdBiliLiveCancel,
						CQBotCmdTiktokLive, CQBotCmdTiktokLiveCancel))
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
			}
		}
	}
}

func (c *CQBotSink) Subscribe(gId, cId uint64, mask int) {
	c.lock.Lock()
	defer c.lock.Unlock()

	flag := c.flag[gId]
	if flag&mask != 0 && c.table[gId] == cId {
		_ = c.bot.SendGuildMsg(gId, cId, "当前频道已经设置订阅")
	} else {
		c.flag[gId] = flag | mask
		c.table[gId] = cId
		_ = c.bot.SendGuildMsg(gId, cId, "订阅成功")
	}
}

func (c *CQBotSink) Unsubscribe(gId, cId uint64, mask int) {
	c.lock.Lock()
	defer c.lock.Unlock()

	flag := c.flag[gId]
	if flag&mask == 1 {
		flag &= ^mask
		c.flag[gId] = flag
		_ = c.bot.SendGuildMsg(gId, cId, "取消成功")
	} else {
		_ = c.bot.SendGuildMsg(gId, cId, "当前频道未订阅消息")
	}
	if flag == 0 {
		delete(c.table, gId)
	}
}
