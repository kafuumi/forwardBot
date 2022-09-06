package push

import (
	"errors"
	"fmt"
	"forwardBot/req"
	"strings"

	"github.com/tidwall/gjson"
)

// DingTalk 钉钉群机器人
type DingTalk struct {
	webhook string //webhook地址
	secret  string //加签密钥
	client  *req.C
}

func NewDingTalk(webhook, secret string) *DingTalk {
	return &DingTalk{
		webhook: webhook,
		secret:  secret,
		client:  req.New(5),
	}
}

func (d *DingTalk) PushMsg(m *Msg) error {
	text := strings.Builder{}
	text.WriteString(m.Times.Format("2006-01-02 15:04") + "\n\n")
	text.WriteString(fmt.Sprintf("%s %s\n\n", m.Author, m.Title))
	text.WriteString(m.Text + "\n\n")
	if m.Src != "" {
		text.WriteString(fmt.Sprintf("<a>%s</a>\n\n", m.Src))
		text.WriteString(fmt.Sprintf("[点击打开链接](%s)\n\n", m.Src))
	}
	if m.Img != "" {
		text.WriteString(fmt.Sprintf("![封面](%s)", m.Img))
	}
	body := req.D{
		{"msgtype", "markdown"},
		{"markdown", req.D{
			{"title", fmt.Sprintf("%s%s", m.Author, m.Title)},
			{"text", text.String()},
		}},
	}
	timestamp, sign := signPusher(d.secret)
	resp, err := d.client.Post(fmt.Sprintf("%s&timestamp=%s&sign=%s", d.webhook, timestamp, sign),
		nil, strings.NewReader(body.Json()),
		req.E{Name: "Content-Type", Value: "application/json"})
	if err != nil {
		return err
	}
	if resp.Len() == 0 {
		return ErrEmptyResp
	}
	data := gjson.ParseBytes(resp.Bytes())
	code := data.Get("errcode").Int()
	if code != 0 {
		return errors.New(data.Get("errmsg").String())
	}
	return nil
}
