package main

import (
	"bufio"
	"context"
	"fmt"
	"forwardBot"
	"forwardBot/push"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"os/signal"
	"path"
	"runtime"
	"time"
)

const (
	Version = "bot: v0.1.1"
)

var (
	cfg    *Config
	logger *logrus.Logger
)

func main() {
	fmt.Println("Version: ", Version)
	cfgFile, err := os.Open(path.Dir(os.Args[0]) + "/config.yaml")
	if err != nil {
		fmt.Printf("[Error] 打开配置文件失败：%v\n", err)
		panic(err)
	}
	cfg, err = ReadCfg(cfgFile)
	if err != nil {
		fmt.Printf("[Error] 读取配置文件失败：%v\n", err)
		panic(err)
	}

	logFile, err := os.Create(fmt.Sprintf("%s.log", time.Now().Format("200601021504")))
	if err != nil {
		fmt.Printf("[Error] 创建日志文件失败：%v\n", err)
		panic(err)
	}
	logWriter := bufio.NewWriter(logFile)
	SetUpLogger(os.Stdout, logWriter)
	forwardBot.SetLogger(logger)
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
		logger.Warn("未配置CQBot, 不推送消息至QQ")
	} else {
		cqBot = forwardBot.NewCQBotSink(cfg.CQBot.Host, cfg.CQBot.Token, cfg.CQBot.BufSize)
		bot.AppendSink(cqBot)
	}

	go func() {
		if cqBot != nil {
			err := cqBot.Listen(ctx)
			if err != nil {
				logger.WithField("err", err).Error("CQBot出现错误")
			}
		}
	}()

	go bot.Run(ctx)
	exits := make(chan os.Signal)
	signal.Notify(exits, os.Interrupt, os.Kill)
	<-exits
	cancel()
	_ = logWriter.Flush()
	_ = logFile.Close()
	logger.Info("程序退出")
}

func SetUpLogger(writers ...io.Writer) {
	//设置日志
	formatter := &logrus.TextFormatter{
		DisableColors:    true,
		TimestampFormat:  "01-02 15:04:05",
		QuoteEmptyFields: true,
		CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
			filename := path.Base(f.File)
			return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)

		},
	}
	logLevel := logrus.DebugLevel
	switch cfg.LogLevel {
	case "Trace":
		logLevel = logrus.TraceLevel
	case "Debug":
		logLevel = logrus.DebugLevel
	case "Info":
		logLevel = logrus.InfoLevel
	case "Warn":
		logLevel = logrus.WarnLevel
	case "Error":
		logLevel = logrus.ErrorLevel
	case "Fatal":
		logLevel = logrus.FatalLevel
	case "Panic":
		logLevel = logrus.PanicLevel
	}
	logger = logrus.New()
	logger.SetLevel(logLevel)
	logger.SetFormatter(formatter)
	logger.SetReportCaller(true)
	logger.SetOutput(io.MultiWriter(writers...))
}

func BiliLiveSource() forwardBot.Source {
	if len(cfg.Bili.Live) == 0 {
		logger.Warn("不监控B站开播状态")
		return nil
	}
	return forwardBot.NewBiliLiveSource(cfg.Bili.Live)
}

func BiliDynamicSource() forwardBot.Source {
	if len(cfg.Bili.Dynamic) == 0 {
		logger.Warn("不监控B站动态")
		return nil
	}
	return forwardBot.NewBiliDynamicSource(cfg.Bili.Dynamic)
}

func TikTokLiveSource() forwardBot.Source {
	if len(cfg.Tiktok.Users) == 0 {
		logger.Warn("不监控抖音开播状态")
		return nil
	}
	return forwardBot.NewTiktokLiveSource(cfg.Tiktok.Nonce, cfg.Tiktok.Signature, cfg.Tiktok.Users)
}

func DingTalkSink() forwardBot.Sink {
	if cfg.DingTalk.Webhook == "" {
		logger.Warn("未配置钉钉，不推送消息")
		return nil
	}
	return forwardBot.NewPushSink(push.NewDingTalk(cfg.DingTalk.Webhook, cfg.DingTalk.Secret))
}
