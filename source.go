package forwardBot

import (
	"context"
	"forwardBot/push"
)

type Source interface {
	// Send 向ch中发送信息
	Send(ctx context.Context, ch chan<- *push.Msg)
}
