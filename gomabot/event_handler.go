package gomabot

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var (
	reCodefence = regexp.MustCompile("(?m)^```([a-z]+)?\n([^`]+\n)^```")
)

func (mb *MatrixBot) OnEventStateMember(ctx context.Context, evt *event.Event) {
	if evt.GetStateKey() == mb.Client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
		_, err := mb.Client.JoinRoomByID(ctx, evt.RoomID)
		if err != nil {
			log.Error().Err(err).
				Str("room_id", evt.RoomID.String()).
				Str("inviter", evt.Sender.String()).
				Msg("Failed to join room after invite")
			return
		}

		log.Info().
			Str("room_id", evt.RoomID.String()).
			Str("inviter", evt.Sender.String()).
			Msg("Joined room after invite")
	}
}

func (mb *MatrixBot) OnEventMessage(ctx context.Context, evt *event.Event) {
	err := mb.Client.MarkRead(ctx, evt.RoomID, evt.ID)
	if err != nil {
		log.Error().Err(err).Msg("failed to mark message as read")

	}

	log.Info().
		Str("sender", evt.Sender.String()).
		Str("type", evt.Type.String()).
		Str("id", evt.ID.String()).
		Str("body", evt.Content.AsMessage().Body).
		Msg("Received message")

	// ignore own messages
	if mb.Client.UserID == evt.Sender {
		return
	}

	for _, handler := range mb.Handlers {
		messageEventContent := evt.Content.AsMessage()

		msg := messageEventContent.Body
		if len(messageEventContent.FormattedBody) > 0 {
			msg = messageEventContent.FormattedBody

		}
		msg = strings.TrimSpace(msg)

		if !handler.Pattern.MatchString(msg) {
			continue
		}

		go func(ctx context.Context, senderID id.UserID, roomID id.RoomID, msg string) {
			handlerMsg, err := handler.Handler(ctx, senderID, roomID, msg)
			if err != nil {
				handlerMsg = "unknown error occured executing command"
				log.Error().Err(err).Msg("failed to call command handler")
			}

			err = mb.sendMessage(ctx, senderID, roomID, handlerMsg)
			if err != nil {
				log.Error().Err(err).Msg("failed to send message")
				return
			}
		}(ctx, evt.Sender, evt.RoomID, msg)

		break
	}
}

func (mb *MatrixBot) sendMessage(ctx context.Context, senderID id.UserID, roomID id.RoomID, msg string) error {
	_ = senderID

	var isHTMLMessage bool
	htmlMsg := msg
	if strings.Contains(msg, "```") {
		isHTMLMessage = true

		// convert codeblocks to HTML
		matchIndexes := reCodefence.FindAllStringSubmatchIndex(htmlMsg, -1)

		for i := len(matchIndexes) - 1; i >= 0; i-- {
			idxs := matchIndexes[i]
			fmt.Println("at index", idxs)

			textBefore := htmlMsg[:idxs[0]]
			textAfter := htmlMsg[idxs[1]:]
			codeblock := strings.ReplaceAll(htmlMsg[idxs[4]:idxs[5]], "\n", "||NEWLINE||")

			var codeClass string
			if idxs[2] != -1 {
				codeClass = fmt.Sprintf(` class="language-%s"`, htmlMsg[idxs[2]:idxs[3]])
			}

			htmlMsg = fmt.Sprintf("%s<pre><code%s>%s</code></pre>%s", textBefore, codeClass, codeblock, textAfter)
		}

	}

	msgEventContent := event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    msg,
	}

	// indicate it's formatted as HTML
	if isHTMLMessage {
		msgEventContent.Format = event.FormatHTML
		msgEventContent.FormattedBody = strings.ReplaceAll(
			strings.ReplaceAll(htmlMsg, "\n", "<br/>"),
			"||NEWLINE||",
			"\n",
		)
	}

	_, err := mb.Client.SendMessageEvent(ctx, roomID, event.EventMessage, &msgEventContent)
	return err
}
