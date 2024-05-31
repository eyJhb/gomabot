package gomabot

import (
	"context"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const (
	ContextKey = "matrixbot-vars"
)

var (
	reCodefence = regexp.MustCompile("(?m)^```([a-z]+)?\n([^`]+\n)^```")
)

func (mb *MatrixBot) AddEventHandler(pattern string, handlerFunc CommandHandlerFunc) {
	mb.Handlers = append(mb.Handlers, CommandHandler{
		Pattern: *regexp.MustCompile(pattern),
		Handler: handlerFunc,
	})
}

func (mb *MatrixBot) AddEventHandlerFormattedBody(pattern string, handlerFunc CommandHandlerFunc) {
	mb.Handlers = append(mb.Handlers, CommandHandler{
		Pattern:            *regexp.MustCompile(pattern),
		Handler:            handlerFunc,
		MatchFormattedBody: true,
	})
}

func (mb *MatrixBot) OnEventStateMember(ctx context.Context, evt *event.Event) {
	if evt.GetStateKey() == mb.Client.UserID.String() && evt.Content.AsMember().Membership == event.MembershipInvite {
		var err error
		if mb.RoomjoinHandler != nil {
			err = mb.RoomjoinHandler(ctx, mb.Client, evt)
		}

		log.Info().
			Str("room_id", evt.RoomID.String()).
			Str("inviter", evt.Sender.String()).
			Errs("room_join_handler", []error{err}).
			Msg("room join invite")

		if err == nil {
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
}

func (mb *MatrixBot) OnEventMessage(ctx context.Context, evt *event.Event) {
	// ignore own messages
	if mb.Client.UserID == evt.Sender {
		return
	}

	// log messages
	log.Info().
		Str("sender", evt.Sender.String()).
		Str("type", evt.Type.String()).
		Str("id", evt.ID.String()).
		Str("body", evt.Content.AsMessage().Body).
		Msg("Received message")

	// ignore messages older than 1 minute
	if time.Now().Sub(time.Unix(evt.Timestamp, 0)) > time.Minute {
		return
	}

	for _, handler := range mb.Handlers {
		me := evt.Content.AsMessage()

		body := me.Body
		if handler.MatchFormattedBody {
			if handler.Pattern.MatchString(me.FormattedBody) {
				body = me.FormattedBody
			}
		}

		if !handler.Pattern.MatchString(body) {
			continue
		}

		// extract potential groupnames
		reGroupNames := handler.Pattern.SubexpNames()
		vars := make(map[string]string)
		if len(reGroupNames) > 0 {
			for _, match := range handler.Pattern.FindAllStringSubmatch(body, -1) {
				for groupIdx, group := range match {
					name := reGroupNames[groupIdx]
					if name == "" {
						continue
					}

					vars[name] = group
				}
			}
		}

		ctx = context.WithValue(ctx, ContextKey, vars)

		// mark as read if it matches a handler
		err := mb.Client.MarkRead(ctx, evt.RoomID, evt.ID)
		if err != nil {
			log.Error().Err(err).Msg("failed to mark message as read")
		}

		go func(ctx context.Context, client *mautrix.Client, evt *event.Event) {
			err := handler.Handler(ctx, client, evt)
			if err != nil {
				log.Error().Err(err).Msg("failed to call command handler")

				if mb.HandlerSendErrors {
					_, err = mb.Client.SendText(ctx, evt.RoomID, err.Error())
					log.Error().Err(err).Msg("failed to send error to channel")
				}
			}
		}(ctx, mb.Client, evt)

		break
	}
}

func ExtractVars(ctx context.Context) map[string]string {
	v := ctx.Value(ContextKey)
	if mapv, ok := v.(map[string]string); ok {
		return mapv
	}

	return make(map[string]string)
}
