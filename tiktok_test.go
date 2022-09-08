package forwardBot

import (
	"bytes"
	"forwardBot/req"
	"github.com/tidwall/gjson"
	"net/url"
	"testing"
)

func TestGetLiveInfo(t *testing.T) {
	client := req.New(10)
	nonce := ""
	signature := ""
	client.SetCookies("__ac_nonce", nonce)
	client.SetCookies("__ac_signature", signature)
	client.SetCookies("__ac_referer", "https://live.douyin.com/")
	resp, err := client.Get(tiktokLiveUrl+"804284713107", nil, nil)
	if err != nil {
		t.Log(err)
	}
	b := resp.Bytes()
	var start, end int
	start = bytes.Index(b, []byte(startFlag))
	if start < 0 {
		t.Log(err)
	}
	b = b[start+len(startFlag):]
	end = bytes.Index(b, []byte(endFlag))
	if end < 0 {
		t.Log(err)
	}
	b = b[:end]
	jsonStr, err := url.QueryUnescape(string(b))
	if err != nil {
		t.Log(err)
	}
	roomInfo := gjson.Get(jsonStr, "app.initialState.roomStore.roomInfo")
	if !roomInfo.Exists() {
		t.Log(err)
	}
	t.Log(roomInfo.String())
}
