package main

import (
	"context"
	"fmt"
	"forwardBot"
	"forwardBot/push"
	"os"
	"os/signal"
	"path"
)

var (
	cfg *Config
)

func main() {
	cfgFile, err := os.Open(path.Dir(os.Args[0]) + "/config_ignore.yaml")
	if err != nil {
		fmt.Printf("打开配置文件失败：%v\n", err)
		panic(err)
	}
	cfg, err = ReadCfg(cfgFile)
	if err != nil {
		fmt.Printf("读取配置文件失败：%v\n", err)
		panic(err)
	}
	bot := forwardBot.NewBot(cfg.MsgBuf)
	bot.AppendSource(
		BiliLiveSource(),
		BiliDynamicSource(),
		TikTokLiveSource(),
	)
	bot.AppendSink(DingTalkSink())

	ctx, cancel := context.WithCancel(context.Background())
	var cqBot *forwardBot.CQBotSink
	if cfg.CQBot.Host == "" {
		fmt.Println("不推送消息至qq")
	} else {
		cqBot = forwardBot.NewCQBotSink(cfg.CQBot.Host, cfg.CQBot.Token, cfg.CQBot.BufSize)
		bot.AppendSink(cqBot)
	}

	go func() {
		if cqBot != nil {
			err := cqBot.Listen(ctx)
			if err != nil {
				fmt.Println(err)
			}
		}
	}()

	go bot.Run(ctx)
	exits := make(chan os.Signal)
	signal.Notify(exits, os.Interrupt, os.Kill)
	<-exits
	fmt.Println("断开连接")
	cancel()
}

func BiliLiveSource() forwardBot.Source {
	if len(cfg.Bili.Live) == 0 {
		fmt.Println("不监控b站直播")
		return nil
	}
	return forwardBot.NewBiliLiveSource(cfg.Bili.Live)
}

func BiliDynamicSource() forwardBot.Source {
	if len(cfg.Bili.Dynamic) == 0 {
		fmt.Println("不监控b站动态")
		return nil
	}
	return forwardBot.NewBiliDynamicSource(cfg.Bili.Dynamic)
}

func TikTokLiveSource() forwardBot.Source {
	if len(cfg.Tiktok.Users) == 0 {
		fmt.Println("不监控抖音直播间")
		return nil
	}
	return forwardBot.NewTiktokLiveSource(cfg.Tiktok.Nonce, cfg.Tiktok.Signature, cfg.Tiktok.Users)
}

func DingTalkSink() forwardBot.Sink {
	if cfg.DingTalk.Webhook == "" {
		fmt.Println("不推送消息至钉钉")
		return nil
	}
	return forwardBot.NewPushSink(push.NewDingTalk(cfg.DingTalk.Webhook, cfg.DingTalk.Secret))
}
