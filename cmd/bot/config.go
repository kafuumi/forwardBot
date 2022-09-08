package main

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io"
)

type BiliCfg struct {
	Live    []int64 `yaml:"live"`
	Dynamic []int64 `yaml:"dynamic"`
}

type TiktokCfg struct {
	Nonce     string   `yaml:"nonce"`
	Signature string   `yaml:"signature"`
	Users     []string `yaml:"users"`
}

type DingTalkCfg struct {
	Webhook string `yaml:"webhook"`
	Secret  string `yaml:"secret"`
}

type CQBotCfg struct {
	Host    string `yaml:"host"`
	Token   string `yaml:"token"`
	BufSize int    `yaml:"bufSize"`
}

type Config struct {
	MsgBuf   int         `yaml:"msgBuf"`
	Bili     BiliCfg     `yaml:"bili"`
	Tiktok   TiktokCfg   `yaml:"tiktok"`
	DingTalk DingTalkCfg `yaml:"dingTalk,omitempty"`
	CQBot    CQBotCfg    `yaml:"cqBot,omitempty"`
}

func ReadCfg(reader io.Reader) (*Config, error) {
	var config Config
	err := yaml.NewDecoder(reader).Decode(&config)
	if err != nil {
		return nil, errors.Wrap(err, "read config")
	}
	return &config, nil
}
