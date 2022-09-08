package qbot

import "strings"

// CQBotCmd 机器人支持的命令
// 格式 /命令 空格分割的参数
type CQBotCmd struct {
	Cmd    string
	Params []string
}

func ParseCQBotCmd(src string) *CQBotCmd {
	//去除首尾空格
	src = strings.TrimSpace(src)
	if len(src) == 0 {
		return nil
	}
	splits := strings.Fields(src)
	//不是命令格式
	if splits[0][0] != '/' {
		return nil
	}
	return &CQBotCmd{
		Cmd:    splits[0],
		Params: splits[1:],
	}
}
