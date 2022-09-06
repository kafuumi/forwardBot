package push

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"forwardBot/req"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// DingTalk 钉钉群机器人
type DingTalk struct {
	webhook string //webhook地址
	secret  string //加签密钥
}

func NewDingTalk(webhook, secret string) *DingTalk {
	return &DingTalk{
		webhook: webhook,
		secret:  secret,
	}
}

var (
	markdownTable = map[rune]string{
		'*':  `\*`,
		'[':  `\[`,
		']':  `\]`,
		'(':  `\(`,
		')':  `\)`,
		'\n': "\n\n",
		'>':  `\>`,
		'-':  `\-`,
	}
)

func escapeMarkdown(src string) string {
	res := strings.Builder{}
	for _, c := range src {
		if cc, ok := markdownTable[c]; ok {
			res.WriteString(cc)
		} else {
			res.WriteRune(c)
		}
	}
	return res.String()
}

func (d *DingTalk) PushMsg(m *Msg) error {
	text := strings.Builder{}
	text.WriteString(m.Times.Format("2006-01-02 15:04") + "\n\n")
	text.WriteString(fmt.Sprintf("%s %s\n\n", m.Author, m.Title))
	text.WriteString(escapeMarkdown(m.Text) + "\n\n")
	if m.Src != "" {
		text.WriteString(fmt.Sprintf("<a>%s</a>\n\n", m.Src))
		text.WriteString(fmt.Sprintf("[点击打开链接](%s)\n\n", m.Src))
	}
	if len(m.Img) != 0 {
		for i := range m.Img {
			text.WriteString(fmt.Sprintf("![封面](%s)", m.Img[i]))
			if i != len(m.Img)-1 {
				text.WriteString("\n\n")
			}
		}
	}
	body := req.D{
		{"msgtype", "markdown"},
		{"markdown", req.D{
			{"title", fmt.Sprintf("%s%s", m.Author, m.Title)},
			{"text", text.String()},
		}},
	}
	timestamp, sign := signPusher(d.secret)
	resp, err := req.Post(fmt.Sprintf("%s&timestamp=%s&sign=%s", d.webhook, timestamp, sign),
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

// 加签
func signPusher(secret string) (timestamp string, sign string) {
	timestamp = strconv.FormatInt(time.Now().UnixMilli(), 10)
	strToSign := []byte(timestamp + "\n" + secret)

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(strToSign)
	signData := mac.Sum(nil)
	sign = url.QueryEscape(base64.StdEncoding.EncodeToString(signData))
	return
}
