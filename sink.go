package forwardBot

import (
	"forwardBot/push"
)

type Sink interface {
	Receive(msg *push.Msg) error
}

type PushSink struct {
	pusher push.Pusher
}

func NewPushSink(p push.Pusher) *PushSink {
	return &PushSink{pusher: p}
}

func (p *PushSink) Receive(msg *push.Msg) error {
	return p.pusher.PushMsg(msg)
}
