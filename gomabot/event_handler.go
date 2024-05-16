package gomabot

import (
	"context"
	"regexp"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
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
		me := evt.Content.AsMessage()
		if !handler.Pattern.MatchString(me.Body) {
			continue
		}

		// extract potential groupnames
		reGroupNames := handler.Pattern.SubexpNames()
		vars := make(map[string]string)
		if len(reGroupNames) > 0 {
			for _, match := range handler.Pattern.FindAllStringSubmatch(me.Body, -1) {
				for groupIdx, group := range match {
					name := reGroupNames[groupIdx]
					if name == "" {
						continue
					}

					vars[name] = group
				}
			}
		}

		ctx = context.WithValue(ctx, "matrixbot-vars", vars)

		// mark as read if it matches a handler
		err := mb.Client.MarkRead(ctx, evt.RoomID, evt.ID)
		if err != nil {
			log.Error().Err(err).Msg("failed to mark message as read")
		}

		go func(ctx context.Context, client *mautrix.Client, evt *event.Event) {
			err := handler.Handler(ctx, client, evt)
			if err != nil {
				log.Error().Err(err).Msg("failed to call command handler")
			}
		}(ctx, mb.Client, evt)

		break
	}
}
