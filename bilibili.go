package forwardBot

import (
	"bytes"
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/req"
	"github.com/pkg/errors"
	"time"

	"github.com/tidwall/gjson"
)

const (
	infoUrl       = "https://api.bilibili.com/x/space/acc/info"
	liveUrlPrefix = "https://live.bilibili.com/"

	interval   = time.Duration(30) * time.Second
	reqTimeout = 5
)

var (
	ErrEmptyRespData = errors.New("empty data") //http响应体为空
)

// BiliLiveSource 获取b站直播间是否开播状态
type BiliLiveSource struct {
	uid    []int64
	living map[int64]bool
	client *req.C
}

type BaseInfo struct {
	Code int
	Msg  string
}

// LiveInfo 直播间信息
type LiveInfo struct {
	BaseInfo
	Mid        int64  //uid
	Uname      string //昵称
	LiveStatus bool   //是否开播
	RoomId     int    //房间号
	Title      string //房间标题
	Cover      string //封面
}

func NewBiliLiveSource(uid []int64) *BiliLiveSource {
	return &BiliLiveSource{
		uid:    append([]int64{}, uid...),
		living: make(map[int64]bool),
		client: req.New(reqTimeout),
	}
}

func checkResp(buf *bytes.Buffer) (result *gjson.Result, err error) {
	if buf == nil || buf.Len() == 0 {
		return nil, ErrEmptyRespData
	}
	r := gjson.ParseBytes(buf.Bytes())
	return &r, nil
}

// 获取用户信息
func getInfo(client *req.C, mid int64) (info *LiveInfo, err error) {
	body, err := client.Get(infoUrl, req.D{{"mid", mid}}, nil)
	if err != nil {
		return nil, err
	}
	result, err := checkResp(body)
	if err != nil {
		return nil, errors.Wrap(err, "read bili resp data")
	}
	info = &LiveInfo{}
	code := result.Get("code").Int()
	if code != 0 {
		info.Code = int(code)
		info.Msg = result.Get("msg").String()
		return info, nil
	}
	data := result.Get("data")
	info.Mid = mid
	info.Uname = data.Get("name").String()

	liveRoom := data.Get("live_room")
	if !liveRoom.Exists() {
		info.Code = 400
		info.Msg = "响应体中无live_room字段"
		return info, nil
	}
	info.LiveStatus = liveRoom.Get("liveStatus").Int() == 1
	info.RoomId = int(liveRoom.Get("roomid").Int())
	info.Title = liveRoom.Get("title").String()
	info.Cover = liveRoom.Get("cover").String()
	return info, nil
}

func (b *BiliLiveSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
		case now := <-ticker.C:
			for _, id := range b.uid {
				info, err := getInfo(b.client, id)
				if err != nil {
					continue
				}
				//当前开播状态和已经记录的开播状态相同，说明已经发送过消息
				if info.LiveStatus == b.living[info.Mid] {
					continue
				}
				msg := &push.Msg{
					Times:  now,
					Author: info.Uname,
				}
				if info.Code != 0 {
					msg.Title = "获取直播间状态失败"
					msg.Text = fmt.Sprintf("[error] %s, code=%d", info.Msg, info.Code)
				} else {
					if info.LiveStatus {
						//开播
						b.living[info.Mid] = true
						msg.Title = "开播了"
						msg.Text = fmt.Sprintf("标题：\"%s\"", info.Title)
						msg.Img = info.Cover
						msg.Src = fmt.Sprintf("%s%d", liveUrlPrefix, info.RoomId)
					} else {
						//下播
						b.living[info.Mid] = false
						msg.Title = "下播了"
						msg.Text = "😭😭😭"
					}
				}
				ch <- msg
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
