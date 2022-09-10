package forwardBot

import (
	"bytes"
	"context"
	"fmt"
	"forwardBot/push"
	"forwardBot/req"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"strconv"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

const (
	infoUrl          = "https://api.bilibili.com/x/space/acc/info"
	roomInfoUrl      = "https://api.live.bilibili.com/xlive/web-room/v1/index/getInfoByRoom"
	liveUrlPrefix    = "https://live.bilibili.com/"
	spaceUrl         = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space"
	dynamicUrlPrefix = "https://t.bilibili.com/"
	videoUrlPrefix   = "https://www.bilibili.com/video/"
	articleUrlPrefix = "https://www.bilibili.com/read/cv"
	musicUrlPrefix   = "https://www.bilibili.com/audio/au"
	interval         = 10 * time.Second
	waitInterval     = 100 * time.Millisecond
)

var (
	ErrEmptyRespData = errors.New("empty data") //http响应体为空
	//liveInfo对象池
	liveInfoPool = &sync.Pool{
		New: func() any {
			return new(LiveInfo)
		},
	}
	//DynamicInfo对象池
	dynInfoPool = &sync.Pool{
		New: func() any {
			return new(DynamicInfo)
		},
	}
)

var _ Source = (*BiliLiveSource)(nil)

// BiliLiveSource 获取b站直播间是否开播状态
type BiliLiveSource struct {
	room   []int
	living map[int]bool
}

// LiveInfo 直播间信息
type LiveInfo struct {
	Mid        int64  //uid
	MidStr     string //字符串形式的uid，抖音的uid和房间号id较长，可能会超范围，作为扩展用，b站返回的数据中为空字符串
	Uname      string //昵称
	LiveStatus bool   //是否开播
	RoomId     int    //房间号
	RoomIdStr  string
	Title      string //房间标题
	Area       string //直播间分区
	Cover      string //封面
}

func (l *LiveInfo) Reset() {
	l.Mid = 0
	l.MidStr = ""
	l.Uname = ""
	l.LiveStatus = false
	l.RoomId = 0
	l.RoomIdStr = ""
	l.Title = ""
	l.Cover = ""
}

func NewBiliLiveSource(room []int) *BiliLiveSource {
	logger.WithFields(logrus.Fields{
		"room": room,
	}).Info("[BiliLive]监控b站开播状态")
	return &BiliLiveSource{
		room:   append([]int{}, room...),
		living: make(map[int]bool),
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
	if !d.Exists() {
		return nil, 400, "没有data字段"
	}
	return &d, 0, ""
}

// 获取直播间信息,此处的id为直播间号,可以是短号
func getRoomInfo(roomId int) (info *LiveInfo, err error) {
	body, err := req.Get(roomInfoUrl, req.D{{"room_id", roomId}})
	if err != nil {
		return nil, err
	}
	result, err := checkResp(body)
	if err != nil {
		return nil, errors.Wrap(err, "read bili resp data fail")
	}
	data, code, msg := checkBiliData(result)
	if code != 0 {
		return nil, errors.New(fmt.Sprintf("code=%d,msg=%s", code, msg))
	}
	info = liveInfoPool.Get().(*LiveInfo)
	roomInfo := data.Get("room_info")
	if !roomInfo.Exists() || !roomInfo.IsObject() {
		logger.WithFields(logrus.Fields{
			"roomId": roomId,
			"resp":   data.String(),
		}).Error("[BiliLive]获取data.room_info失败")
		return nil, errors.New("[BiliLive]获取data.room_info失败")
	}
	userInfo := data.Get("anchor_info.base_info")
	if !userInfo.Exists() || !userInfo.IsObject() {
		logger.WithFields(logrus.Fields{
			"roomId": roomId,
			"resp":   data.String(),
		}).Error("[BiliLive]获取data.anchor_info.base_info失败")
		return nil, errors.New("[BiliLive]获取data.anchor_info.base_info失败")
	}

	status := roomInfo.Get("live_status")
	if !status.Exists() {
		logger.WithFields(logrus.Fields{
			"roomId":   roomId,
			"roomInfo": roomInfo.String(),
		}).Error("[BiliLive]获取room_info.live_status失败")
		return nil, errors.New("[BiliLive]获取room_info.live_status失败")
	}
	uname := userInfo.Get("uname")
	if !uname.Exists() {
		logger.WithFields(logrus.Fields{
			"roomId":   roomId,
			"userInfo": userInfo.String(),
		}).Error("[BiliLive]获取uname失败")
		return nil, errors.New("[BiliLive]获取uname失败")
	}
	info.Mid = roomInfo.Get("uid").Int()
	info.Uname = uname.String()
	info.LiveStatus = status.Int() == 1
	info.RoomId = roomId
	if !info.LiveStatus {
		return info, nil
	}
	info.Title = roomInfo.Get("title").String()
	info.Area = fmt.Sprintf("%s-%s",
		roomInfo.Get("parent_area_name").String(),
		roomInfo.Get("area_name").String())
	if len(info.Area) < 3 {
		info.Area = ""
		logger.WithFields(logrus.Fields{
			"roomId":   roomId,
			"roomInfo": roomInfo.String(),
		}).Warn("获取直播间分区失败")
	}
	info.Cover = roomInfo.Get("cover").String()
	return info, nil
}

func (b *BiliLiveSource) sendInfo(id int, now time.Time, ch chan<- *push.Msg) bool {
	info, err := getRoomInfo(id)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"uid": id,
			"err": err,
		}).Error("[BiliLive]获取开播状态失败")
		return false
	}
	//当前开播状态和已经记录的开播状态相同，说明已经发送过消息
	if info.LiveStatus == b.living[id] {
		logger.WithFields(logrus.Fields{
			"id":     info.Mid,
			"living": info.LiveStatus,
		}).Debug("[BiliLive]开播状态未改变")
		info.Reset()
		liveInfoPool.Put(info)
		return false
	}
	msg := &push.Msg{
		Times:  now,
		Flag:   BiliLiveMsg,
		Author: info.Uname,
	}

	b.living[id] = info.LiveStatus
	if info.LiveStatus {
		//开播
		msg.Title = "开播了"
		if info.Area != "" {
			msg.Text = fmt.Sprintf("标题：\"%s\"\n分区：\"%s\"", info.Title, info.Area)
		} else {
			msg.Text = fmt.Sprintf("标题：\"%s\"", info.Title)
		}
		msg.Img = []string{info.Cover}
		msg.Src = fmt.Sprintf("%s%d", liveUrlPrefix, info.RoomId)
		logger.WithFields(logrus.Fields{
			"id":   id,
			"name": info.Uname,
		}).Debug("[BiliLive]b站直播间开播")
	} else {
		//下播
		msg.Title = "下播了"
		msg.Text = "😭😭😭"
		logger.WithFields(logrus.Fields{
			"id":   id,
			"name": info.Uname,
		}).Debug("[BiliLive]b站直播间下播")
	}
	ch <- msg
	info.Reset()
	liveInfoPool.Put(info)
	return true
}

func (b *BiliLiveSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("[BiliLive]停止监控b站直播间")
			return
		case now := <-ticker.C:
			for _, id := range b.room {
				if !b.sendInfo(id, now, ch) {
					continue
				}
				time.Sleep(waitInterval)
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

// 让编译器检查*BiliDynamicSource实现了Source接口
var _ Source = (*BiliDynamicSource)(nil)

type BiliDynamicSource struct {
	uid       []int64
	lastTable map[int64]int64
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

func (d *DynamicInfo) Reset() {
	d.types = ""
	d.id = ""
	d.text = ""
	d.img = nil
	d.author = ""
	d.src = ""
}

func NewBiliDynamicSource(uid []int64) *BiliDynamicSource {
	logger.WithFields(logrus.Fields{
		"uid": uid,
	}).Info("[BiliDyn]监控b站动态更新")
	return &BiliDynamicSource{
		uid:       uid,
		lastTable: make(map[int64]int64),
	}
}

func (b *BiliDynamicSource) Send(ctx context.Context, ch chan<- *push.Msg) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Info("[BiliDyn]停止b站动态监控")
			return
		case now := <-ticker.C:
			for _, id := range b.uid {
				infos, err := b.space(id, now)
				if err != nil {
					logger.WithFields(logrus.Fields{
						"id":  id,
						"err": err,
					}).Error("[BiliDyn]获取b站动态失败")
					continue
				}
				if len(infos) == 0 {
					logger.WithFields(logrus.Fields{
						"id": id,
					}).Debug("[BiliDyn]无新动态")
				}
				for _, info := range infos {
					logger.WithFields(logrus.Fields{
						"id":    id,
						"name":  info.author,
						"title": info.types,
						"src":   info.src,
					}).Debug("[BiliDyn]更新动态")
					msg := &push.Msg{
						Flag:   BiliDynMsg,
						Times:  info.times,
						Author: info.author,
						Title:  info.types,
						Text:   info.text,
						Img:    info.img,
						Src:    info.src,
					}
					ch <- msg
					info.Reset()
					dynInfoPool.Put(info)
				}
				time.Sleep(waitInterval)
			}
		}
	}
}

// 获取动态
func (b *BiliDynamicSource) space(id int64, now time.Time) (infos []*DynamicInfo, err error) {
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
	dyns := data.Get("items")
	if !dyns.Exists() || !dyns.IsArray() {
		logger.WithFields(logrus.Fields{
			"mid":  id,
			"resp": data.String(),
		}).Error("[BiliDyn]获取items失败")
		return nil, errors.New("不存在data.items字段")
	}
	items := dyns.Array()

	infos = make([]*DynamicInfo, 0, len(items))
	var newest int64
	last := b.lastTable[id]
	if last == 0 {
		last = now.Unix() - int64(interval/time.Second)
	}
	for _, item := range items {
		info := parseDynamic(&item)
		if info != nil {
			if info.types == DynamicTypeLive {
				logger.WithFields(logrus.Fields{
					"mid":    id,
					"author": info.author,
					"types":  info.types,
				}).Debug("[BiliDyn]忽略开播动态")
				continue
			}
			second := info.times.Unix()
			newest = max(newest, second)
			if second > last {
				infos = append(infos, info)
			} else {
				logger.WithFields(logrus.Fields{
					"mid": id,
					"src": info.src,
				}).Debug("[BiliDyn]过滤动态")
				info.Reset()
				dynInfoPool.Put(info)
			}
		} else {
			logger.WithFields(logrus.Fields{
				"id": id,
			}).Warn("[BiliDyn]解析的动态为nil")
		}
	}
	last = max(last, newest)
	b.lastTable[id] = last
	return infos, nil
}

func max[T int64 | int | int32 | int8 | int16](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func parseDynamic(item *gjson.Result) *DynamicInfo {
	types := item.Get("type").String()
	info := dynInfoPool.Get().(*DynamicInfo)
	info.id = item.Get("id_str").String()
	info.src = dynamicUrlPrefix + info.id

	author := item.Get("modules.module_author")
	info.author = author.Get("name").String()
	pubTs := author.Get("pub_ts").Int()
	info.times = time.Unix(pubTs, 0)

	dynamic := item.Get("modules.module_dynamic")
	if !dynamic.Exists() {
		return nil
	}
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
		if origInfo.types == DynamicTypeLive {
			info.types = "分享直播间"
			info.text = fmt.Sprintf("%s\n分享\"%s\"的直播间\n%s", text, origInfo.author, origInfo.text)
		} else {
			info.text = fmt.Sprintf("%s \n转发自：@%s\n%s", text, origInfo.author, origInfo.text)
		}
		info.img = origInfo.img
	case DynamicTypeArticle:
		info.types = "投稿专栏"
		article := dynamic.Get("major.article")
		info.id = strconv.FormatInt(article.Get("id").Int(), 10)
		info.src = articleUrlPrefix + info.id
		desc := article.Get("desc").String()
		title := article.Get("title").String()
		info.text = fmt.Sprintf("%s\n%s", title, desc)
		info.img = []string{article.Get("covers.0").String()}
	case DynamicTypeMusic:
		info.types = "投稿音频"
		music := dynamic.Get("major.music")
		info.id = strconv.FormatInt(music.Get("id").Int(), 10)
		info.src = musicUrlPrefix + info.id
		info.text = music.Get("title").String()
		info.img = []string{music.Get("cover").String()}
	case DynamicTypePGC:
		pgc := dynamic.Get("major.pgc")
		info.text = pgc.Get("title").String()
		info.img = []string{pgc.Get("cover").String()}
	case DynamicTypeLive:
		info.types = DynamicTypeLive
		content := dynamic.Get("major.live_rcmd.content").String()
		if content == "" {
			return nil
		}
		liveInfo := gjson.Get(content, "live_play_info")
		info.text = fmt.Sprintf("标题：\"%s\"", liveInfo.Get("title").String())
		info.img = []string{liveInfo.Get("cover").String()}
	default:
		info.types = "发布动态"
		info.text = "未处理的动态类型"
		logger.WithFields(logrus.Fields{
			"resp": item.String(),
		}).Warn("[BiliDyn]未处理的动态类型")
	}
	return info
}
