package push

import "time"

type Pusher interface {
	PushMsg(m *Msg) error
}

type Msg struct {
	Times  time.Time //时间
	Flag   int       //标志位，用于表示该消息的类型
	Author string    //消息发出者
	Title  string    //消息标题
	Text   string    //消息内容
	Img    []string  //消息中的图片
	Src    string    //消息出处
}
