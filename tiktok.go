package forwardBot

import (
	"bytes"
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/req"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"net/url"
	"time"
)

const (
	startFlag          = `<script id="RENDER_DATA" type="application/json">`
	endFlag            = `</script>`
	tiktokLiveUrl      = "https://live.douyin.com/"
	tiktokLiveShareUrl = "https://webcast.amemv.com/douyin/webcast/reflow/"
)

var _ Source = (*TiktokLiveSource)(nil)

type TiktokLiveSource struct {
	client *req.C
	living map[string]bool
	users  []string
}

func NewTiktokLiveSource(nonce, signature string, users []string) *TiktokLiveSource {
	logger.WithFields(logrus.Fields{
		"users": users,
	}).Info("[tiktok]监控抖音直播间开播状态")
	ts := new(TiktokLiveSource)
	ts.client = req.New(10)
	ts.client.SetCookies("__ac_nonce", nonce)
	ts.client.SetCookies("__ac_signature", signature)
	ts.client.SetCookies("__ac_referer", "https://live.douyin.com/")
	ts.living = make(map[string]bool)
	ts.users = users
	return ts
}

func (t *TiktokLiveSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("[tiktok]停止监控抖音直播间")
			return
		case now := <-ticker.C:
			for _, id := range t.users {
				info, err := t.getLiveInfo(id)
				if err != nil {
					logger.WithFields(logrus.Fields{
						"id":  id,
						"err": err,
					}).Error("[tiktok]获取抖音开播状态失败")
					continue
				}
				if info.LiveStatus == t.living[id] {
					logger.WithFields(logrus.Fields{
						"id":     id,
						"living": info.LiveStatus,
					}).Debug("[tiktok]开播状态未改变")
					info.Reset()
					liveInfoPool.Put(info)
					continue
				}
				t.living[id] = info.LiveStatus
				msg := &push.Msg{
					Times:  now,
					Flag:   TikTokLiveMsg,
					Author: info.Uname,
				}
				if info.LiveStatus {
					//开播
					logger.WithFields(logrus.Fields{
						"id":   id,
						"name": info.Uname,
					}).Debug("[tiktok]抖音开播了")
					msg.Title = "抖音开播了"
					msg.Text = fmt.Sprintf("标题：\"%s\"", info.Title)
					msg.Img = []string{info.Cover}
					msg.Src = fmt.Sprintf("%s%s", tiktokLiveShareUrl, info.RoomIdStr)
				} else {
					//下播
					logger.WithFields(logrus.Fields{
						"id":   id,
						"name": info.Uname,
					}).Debug("[tiktok]抖音下播了")
					msg.Title = "抖音下播了"
					msg.Text = "😭😭😭"
				}
				ch <- msg
				info.Reset()
				liveInfoPool.Put(info)
				time.Sleep(waitInterval)
			}
		}
	}
}

func (t *TiktokLiveSource) getLiveInfo(id string) (info *LiveInfo, err error) {
	resp, err := t.client.Get(tiktokLiveUrl+id, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "request fail")
	}
	b := resp.Bytes()
	var start, end int
	start = bytes.Index(b, []byte(startFlag))
	if start < 0 {
		return nil, errors.New("get info fail(start < 0), signature maybe error")
	}
	b = b[start+len(startFlag):]
	end = bytes.Index(b, []byte(endFlag))
	if end < 0 {
		return nil, errors.New("get info fail(end < 0), signature maybe error")
	}
	b = b[:end]
	jsonStr, err := url.QueryUnescape(string(b))
	if err != nil {
		return nil, errors.Wrap(err, "unescape url fail")
	}
	roomInfo := gjson.Get(jsonStr, "app.initialState.roomStore.roomInfo")
	if !roomInfo.Exists() {
		logger.WithFields(logrus.Fields{
			"mid":  id,
			"resp": jsonStr,
		}).Error("[tiktok]获取roomInfo失败")
		return nil, errors.New("not exists roomInfo object")
	}
	room := roomInfo.Get("room")
	if !room.Exists() {
		logger.WithFields(logrus.Fields{
			"mid":  id,
			"resp": roomInfo.String(),
		}).Error("[tiktok]获取roomInfo.room失败")
		return nil, errors.New("not exists room object")
	}
	anchor := roomInfo.Get("anchor")
	if !anchor.Exists() {
		logger.WithFields(logrus.Fields{
			"mid":  id,
			"resp": roomInfo.String(),
		}).Error("[tiktok]获取roomInfo.anchor失败")
		return nil, errors.New("not exists room object")
	}
	status := room.Get("status")
	if !status.Exists() {
		logger.WithFields(logrus.Fields{
			"mid":  id,
			"resp": room.String(),
		}).Error("[tiktok]获取room.status失败")
		return nil, errors.New("not exists room.status")
	}
	//2为开播
	isLiving := status.Int() == 2
	info = liveInfoPool.Get().(*LiveInfo)
	info.MidStr = anchor.Get("id_str").String()
	info.Uname = anchor.Get("nickname").String()
	info.LiveStatus = isLiving
	//这里的roomId和传入的id会不同，这里的roomId是移动端使用的id，
	//pc网页端有一个web_rid，传入的参数id即是web_rid
	info.RoomIdStr = roomInfo.Get("roomId").String()

	if isLiving {
		info.Title = room.Get("title").String()
		info.Cover = room.Get("cover.url_list.0").String()
	}
	return info, nil
}
