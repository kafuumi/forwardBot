package qbot

import (
	"bytes"
	"fmt"
	"github.com/mailru/easyjson"
	"github.com/tidwall/gjson"
	"strconv"
	"strings"
)

const (
	postTypeMsg  = "message"    //收到消息
	postTypeMeta = "meta_event" //心跳包信息
)

type CQBotMsgType int    //消息类型
type CQBotMsgSubType int //子类型，以后扩展预留

const (
	CQBotGuildMsg CQBotMsgType = iota //频道消息
	CQBotHeartMsg                     //心跳包
)

const (
	CQBotChannelMsg CQBotMsgSubType = iota
)

type CQBotMsg struct {
	Times       int64           //时间戳，单位秒
	MsgType     CQBotMsgType    //消息类型
	SubType     CQBotMsgSubType //子类型，扩展预留
	SourceId    uint64          //消息来源地的id，群号或者频道id
	SubSourceId uint64          //子id，例如子频道id
	SelfId      uint64          //bot在消息来源地中的id
	SenderId    uint64          //消息发送者的id
	Text        string          //消息内容
	MsgId       string          //该消息的id
}

func parseCQBotMsg(src []byte) *CQBotMsg {
	r := gjson.ParseBytes(src)
	//通过是否含有retcode字段判断是否是echo msg
	if r.Get("retcode").Exists() {
		echo := r.Get("echo")
		//存在echo字段，需要回调处理
		if echo.Exists() {
			msg := &EchoMsg{}
			err := easyjson.Unmarshal(src, msg)
			if err != nil {
				//TODO
			}
			go hm.Handle(msg)
		}
		return nil
	}
	postType := r.Get("post_type").String()
	switch postType {
	case postTypeMeta:
		return &CQBotMsg{
			Times:   r.Get("time").Int(),
			MsgType: CQBotHeartMsg,
		}
	case postTypeMsg:
		return cqBotMessageMsg(&r)
	}
	return nil
}

// post_type 为message
func cqBotMessageMsg(r *gjson.Result) *CQBotMsg {
	messageType := r.Get("message_type").String()
	switch messageType {
	case "guild":
		return cqBotGuildMsg(r)
	}
	return nil
}

// 收到频道消息
func cqBotGuildMsg(r *gjson.Result) *CQBotMsg {
	subType := r.Get("sub_type").String()
	if subType != "channel" {
		return nil
	}
	text := r.Get("message").String()
	//不是以at消息开头
	if !strings.HasPrefix(text, "[CQ:at") {
		return nil
	}
	//不是合法的CQCode
	i := strings.IndexByte(text, ']')
	if i < 0 {
		return nil
	}
	cqCode := parseCQCode(text[:i+1])
	selfTinyId := r.Get("self_tiny_id").String()
	//不是at消息，或者不是at机器人
	if cqCode == nil || cqCode.Types != "at" || cqCode.Data["qq"] != selfTinyId {
		return nil
	}
	text = strings.TrimSpace(text[i+1:])
	msg := &CQBotMsg{
		Times:    r.Get("time").Int(),
		MsgType:  CQBotGuildMsg,
		SubType:  CQBotChannelMsg,
		Text:     text,
		SenderId: r.Get("sender.user_id").Uint(),
		MsgId:    r.Get("message_id").String(),
	}
	msg.SourceId, _ = strconv.ParseUint(r.Get("guild_id").String(), 10, 64)
	msg.SubSourceId, _ = strconv.ParseUint(r.Get("channel_id").String(), 10, 64)
	msg.SelfId, _ = strconv.ParseUint(selfTinyId, 10, 64)
	return msg
}

type CQCode struct {
	Types string
	Data  map[string]string
}

func (c *CQCode) String() string {
	res := make([]byte, 0)
	res = append(res, fmt.Sprintf("[CQ:%s,", c.Types)...)
	for k := range c.Data {
		name := escapeCQCode(k)
		value := escapeCQCode(c.Data[k])
		res = append(res, name...)
		res = append(res, '=')
		res = append(res, value...)
		res = append(res, ',')
	}
	res[len(res)-1] = ']'
	return string(res)
}

func unescapeCQCode(src string) string {
	i := strings.IndexByte(src, '&')
	if i < 0 {
		return src
	}
	table := map[string]rune{
		"&amp;": '&',
		"&#91;": '[',
		"&#93;": ']',
		"&#44;": ',',
	}
	b := []byte(src)
	res := strings.Builder{}
	for i >= 0 {
		res.Write(b[:i])
		j := i + 5
		if j > len(b) {
			b = b[i:]
			break
		}
		temp := string(b[i:j])
		if c, ok := table[temp]; ok {
			res.WriteRune(c)
		}
		b = b[j:]
		if len(b) == 0 {
			break
		}
		i = bytes.IndexByte(b, '&')
	}
	if len(b) != 0 {
		res.Write(b)
	}
	return res.String()
}

func escapeCQCode(src string) string {
	table := map[rune]string{
		'&': "&amp;",
		'[': "&#91;",
		']': "&#93;",
		',': "&#44;",
	}
	res := strings.Builder{}
	for _, c := range src {
		if cc, ok := table[c]; ok {
			res.WriteString(cc)
		} else {
			res.WriteRune(c)
		}
	}
	return res.String()
}

func parseCQCode(msg string) *CQCode {
	//去除首尾的'['和']'
	msg = msg[1 : len(msg)-1]
	if !strings.HasPrefix(msg, "CQ:") {
		return nil
	}
	splits := strings.Split(msg[3:], ",")
	types := splits[0]
	cqCode := &CQCode{
		Types: types,
		Data:  make(map[string]string),
	}
	for i := 1; i < len(splits); i++ {
		index := strings.IndexByte(splits[i], '=')
		if index < 0 {
			return nil
		}
		name, value := unescapeCQCode(splits[i][:index]), unescapeCQCode(splits[i][index+1:])
		cqCode.Data[name] = value
	}
	return cqCode
}
