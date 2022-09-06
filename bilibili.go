package forwardBot

import (
	"bytes"
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/req"
	"github.com/pkg/errors"
	"strconv"
	"time"

	"github.com/tidwall/gjson"
)

const (
	infoUrl          = "https://api.bilibili.com/x/space/acc/info"
	liveUrlPrefix    = "https://live.bilibili.com/"
	spaceUrl         = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space"
	dynamicUrlPrefix = "https://t.bilibili.com/"
	videoUrlPrefix   = "https://www.bilibili.com/video/"
	articleUrlPrefix = "https://www.bilibili.com/read/cv"
	musicUrlPrefix   = "https://www.bilibili.com/audio/au"
	interval         = time.Duration(30) * time.Second
)

var (
	ErrEmptyRespData = errors.New("empty data") //http响应体为空
)

// BiliLiveSource 获取b站直播间是否开播状态
type BiliLiveSource struct {
	uid    []int64
	living map[int64]bool
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
	}
}

func checkResp(buf *bytes.Buffer) (result *gjson.Result, err error) {
	if buf == nil || buf.Len() == 0 {
		return nil, ErrEmptyRespData
	}
	r := gjson.ParseBytes(buf.Bytes())
	return &r, nil
}

func checkBiliData(r *gjson.Result) (data *gjson.Result, code int, msg string) {
	code = int(r.Get("code").Int())
	if code != 0 {
		msg = r.Get("msg").String()
		return nil, code, msg
	}
	d := r.Get("data")
	return &d, 0, ""
}

// 获取用户信息
func getInfo(mid int64) (info *LiveInfo, err error) {
	body, err := req.Get(infoUrl, req.D{{"mid", mid}})
	if err != nil {
		return nil, err
	}
	result, err := checkResp(body)
	if err != nil {
		return nil, errors.Wrap(err, "read bili resp data")
	}
	info = &LiveInfo{}
	data, code, msg := checkBiliData(result)
	if code != 0 {
		info.Code = code
		info.Msg = msg
		return info, nil
	}
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
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, id := range b.uid {
				info, err := getInfo(id)
				if err != nil {
					//TODO
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
						msg.Img = []string{info.Cover}
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

const (
	DynamicTypeForward = "DYNAMIC_TYPE_FORWARD"   //转发动态
	DynamicTypeDraw    = "DYNAMIC_TYPE_DRAW"      //带图片动态
	DynamicTypeAV      = "DYNAMIC_TYPE_AV"        //视频
	DynamicTypeWord    = "DYNAMIC_TYPE_WORD"      //纯文本
	DynamicTypeArticle = "DYNAMIC_TYPE_ARTICLE"   //专栏
	DynamicTypeMusic   = "DYNAMIC_TYPE_MUSIC"     //音频
	DynamicTypePGC     = "DYNAMIC_TYPE_PGC"       //分享番剧
	DynamicTypeLive    = "DYNAMIC_TYPE_LIVE_RCMD" //开播推送的动态，不做处理
)

type BiliDynamicSource struct {
	uid []int64
}

type DynamicInfo struct {
	types  string    //动态类型
	id     string    //动态的id，如果是视频，则是bv号
	text   string    //动态内容
	img    []string  //动态中的图片
	author string    //动态作者
	src    string    //动态链接
	times  time.Time //动态发布时间
}

func NewBiliDynamicSource(uid []int64) *BiliDynamicSource {
	return &BiliDynamicSource{
		uid: uid,
	}
}

func (b *BiliDynamicSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			for _, id := range b.uid {
				infos, err := space(id, now)
				if err != nil {
					//TODO
					continue
				}
				for _, info := range infos {
					msg := &push.Msg{
						Times:  info.times,
						Author: info.author,
						Title:  info.types,
						Text:   info.text,
						Img:    info.img,
						Src:    info.src,
					}
					ch <- msg
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func space(id int64, now time.Time) (infos []*DynamicInfo, err error) {
	resp, err := req.Get(spaceUrl, req.D{
		{"offset", ""},
		{"host_mid", id},
		{"timezone_offset", "-480"},
	})
	if err != nil {
		return nil, err
	}

	result, err := checkResp(resp)
	if err != nil {
		return nil, errors.Wrap(err, "read bili resp data")
	}
	data, code, msg := checkBiliData(result)
	if code != 0 {
		return nil, errors.New(msg)
	}
	items := data.Get("items").Array()

	res := make([]*DynamicInfo, 0, len(items))
	for _, item := range items {
		info := parseDynamic(&item)
		if info != nil && now.Unix()-info.times.Unix() <= int64(interval/time.Second) {
			res = append(res, info)
		}
	}
	return res, nil
}

func parseDynamic(item *gjson.Result) *DynamicInfo {
	types := item.Get("type").String()
	info := &DynamicInfo{}
	info.id = item.Get("id_str").String()
	info.src = dynamicUrlPrefix + info.id

	author := item.Get("modules.module_author")
	info.author = author.Get("name").String()
	pubTs := author.Get("pub_ts").Int()
	info.times = time.Unix(pubTs, 0)

	dynamic := item.Get("modules.module_dynamic")
	switch types {
	case DynamicTypeWord:
		info.types = "发布动态"
		info.text = dynamic.Get("desc.text").String()
	case DynamicTypeDraw:
		info.types = "发布动态"
		info.text = dynamic.Get("desc.text").String()
		img := dynamic.Get("major.draw.items").Array()
		for i := range img {
			info.img = append(info.img, img[i].Get("src").String())
		}
	case DynamicTypeAV:
		info.types = "投稿视频"
		archive := dynamic.Get("major.archive")
		info.id = archive.Get("bvid").String()
		info.src = videoUrlPrefix + info.id

		desc := archive.Get("desc").String()
		title := archive.Get("title").String()
		info.text = fmt.Sprintf("%s\n%s", title, desc)
		info.img = []string{archive.Get("cover").String()}
	case DynamicTypeForward:
		info.types = "转发动态"
		text := dynamic.Get("desc.text").String()
		orig := item.Get("orig")
		origInfo := parseDynamic(&orig)
		if origInfo == nil {
			return nil
		}
		info.text = fmt.Sprintf("%s \n转发自：@%s\n%s", text, origInfo.author, origInfo.text)
		info.img = origInfo.img
	case DynamicTypeArticle:
		info.types = "投稿专栏"
		article := dynamic.Get("major.article")
		info.id = strconv.FormatInt(article.Get("id").Int(), 10)
		info.src = articleUrlPrefix + info.id
		desc := article.Get("desc").String()
		title := article.Get("title").String()
		info.text = fmt.Sprintf("%s\n%s", title, desc)
		cover := article.Get("covers.0").String()
		info.img = []string{cover}
	case DynamicTypeMusic:
		info.types = "投稿音频"
		music := dynamic.Get("major.music")
		info.id = strconv.FormatInt(music.Get("id").Int(), 10)
		info.src = musicUrlPrefix + info.id
		info.text = music.Get("title").String()
		cover := music.Get("cover").String()
		info.img = []string{cover}
	case DynamicTypePGC:
		pgc := dynamic.Get("major.pgc")
		info.text = pgc.Get("title").String()
		info.img = []string{pgc.Get("cover").String()}
	case DynamicTypeLive:
		//不处理开播动态
		return nil
	default:
		info.types = "发布动态"
		info.text = "未处理的动态类型"
	}
	return info
}
