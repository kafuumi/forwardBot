package push

import "time"

type Pusher interface {
	PushMsg(m *Msg) error
}

type Msg struct {
	Times  time.Time
	Author string
	Title  string
	Text   string
	Img    []string
	Src    string
}
