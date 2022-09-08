package qbot

import (
	"sync"
)

// EchoMsg CQHttp api调用的回响消息
//
//easyjson:json
type EchoMsg struct {
	Status  string         `json:"status"`
	RetCode int            `json:retcode"`
	Msg     string         `json:"msg"`
	Wording string         `json:"wording"`
	Data    map[string]any `json:"data"`
	Echo    string         `json:"echo"`
}

// EchoHandler 处理对应的echo，如果返回false，则会从table中对应的echo删除
//
//easyjson:skip
type EchoHandler func(msg *EchoMsg) bool

var (
	hm = &handlerManager{
		table: make(map[string]EchoHandler),
	}
)

//easyjson:skip
type handlerManager struct {
	table map[string]EchoHandler
	lock  sync.RWMutex
}

func (h *handlerManager) setHandler(echo string, handler EchoHandler) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.table[echo] = handler
}

func (h *handlerManager) Handle(msg *EchoMsg) {
	echo := msg.Echo
	if echo == "" {
		return
	}
	h.lock.RLock()
	handler := h.table[echo]
	h.lock.RUnlock()

	if handler == nil {
		return
	}
	live := handler(msg)
	if !live {
		h.lock.Lock()
		defer h.lock.Unlock()
		delete(h.table, echo)
	}
}

func SetHandler(echo string, handler EchoHandler) {
	hm.setHandler(echo, handler)
}
