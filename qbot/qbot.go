package qbot

import (
	"context"
	"fmt"
	"forwardBot/push"
	"github.com/pkg/errors"
	"github.com/tencent-connect/botgo"
	"github.com/tencent-connect/botgo/dto"
	"github.com/tencent-connect/botgo/dto/message"
	"github.com/tencent-connect/botgo/event"
	"github.com/tencent-connect/botgo/openapi"
	"github.com/tencent-connect/botgo/token"
	"github.com/tencent-connect/botgo/websocket"
	"strings"
)

var (
	channelIdTable = make(map[string]string) //频道id表，记录不同频道中，推送消息的子频道id
	api            openapi.OpenAPI
	ctx            context.Context
)

const (
	cmdSubscribe   = "/订阅推送"
	cmdUnSubscribe = "/取消订阅"
)

// QQBot QQ频道机器人
type QQBot struct {
	appid uint64
	token string
}

func New(appid uint64, token string) *QQBot {
	return &QQBot{
		appid: appid,
		token: token,
	}
}

func (q *QQBot) Start(_ctx context.Context) error {
	ctx = _ctx
	_token := token.BotToken(q.appid, q.token)
	api = botgo.NewOpenAPI(_token)
	ws, err := api.WS(ctx, nil, "")
	if err != nil {
		return errors.Wrap(err, "QQBot: 创建websocket连接失败")
	}
	intent := websocket.RegisterHandlers(
		atMsgEventHandler(),
		errNotifyHandler(),
		readyHandler())
	return botgo.NewSessionManager().Start(ws, _token, &intent)
}

// at机器人设置将消息推送到指定的子频道中
func atMsgEventHandler() event.ATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		res := message.ParseCommand(data.Content)
		member := data.Member
		var isManager bool
		for _, role := range member.Roles {
			//4为频道主，2为管理员
			if role == "4" || role == "2" {
				isManager = true
				break
			}
		}
		//只有管理员或频道主可以设置
		var content string
		if isManager {
			switch res.Cmd {
			case cmdUnSubscribe:
				delete(channelIdTable, data.GuildID)
				content = "取消成功"
			default:
				channelIdTable[data.GuildID] = data.ChannelID
				content = "设置成功，推送消息将发送至当前子频道"
			}
		} else {
			content = "仅限频主和管理员设置"
		}
		_, err := api.PostMessage(ctx, data.ChannelID, &dto.MessageToCreate{
			MsgID:   data.ID,
			Content: content,
			MessageReference: &dto.MessageReference{
				MessageID: data.ID,
			},
		})
		return err
	}
}

func errNotifyHandler() event.ErrorNotifyHandler {
	return func(err error) {
		fmt.Println(err)
	}
}

func readyHandler() event.ReadyHandler {
	return func(event *dto.WSPayload, data *dto.WSReadyData) {
		fmt.Println(data.User.Username)
		fmt.Println(data.Version)
	}
}

func (q *QQBot) Receive(msg *push.Msg) error {
	qqMsg := &dto.MessageToCreate{}
	text := strings.Builder{}
	text.WriteString(msg.Times.Format("2006-01-02 15:04"))
	text.WriteByte('\n')
	text.WriteString(fmt.Sprintf("%s %s\n", msg.Author, msg.Title))
	text.WriteString(msg.Text)
	qqMsg.Content = text.String()
	if len(msg.Img) != 0 {
		qqMsg.Image = msg.Img[0]
	}

	for _, id := range channelIdTable {
		_, err := api.PostMessage(ctx, id, qqMsg)
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}
