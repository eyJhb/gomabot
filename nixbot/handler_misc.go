package nixbot

import (
	"context"
	"fmt"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (nb *NixBot) CommandHandlerPing(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	elapsedTime := time.Now().Sub(time.UnixMilli(evt.Timestamp))

	_, err := client.SendText(ctx, evt.RoomID, fmt.Sprintf("pong - elapsed time: %s", elapsedTime))
	return err
}

func (nb *NixBot) CommandHandlerEcho(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	content := evt.Content.AsMessage()
	_, err := client.SendText(ctx, evt.RoomID, content.Body)
	return err
}
