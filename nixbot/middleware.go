package nixbot

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/eyJhb/gomabot/gomabot"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (nb *NixBot) MiddlewareTimeout(timeout time.Duration, fn gomabot.CommandHandlerFunc) gomabot.CommandHandlerFunc {
	return func(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
		newCtx, cancelFunc := context.WithTimeout(ctx, timeout)
		defer cancelFunc()

		return fn(newCtx, client, evt)
	}
}

func (nb *NixBot) MiddlewareAdmin(admins []string, fn gomabot.CommandHandlerFunc) gomabot.CommandHandlerFunc {
	return func(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
		for _, admin := range admins {
			if admin == evt.Sender.String() {
				return fn(ctx, client, evt)
			}
		}

		return errors.New("not admin")
	}
}

func (nb *NixBot) MiddlewareResponse(fn gomabot.CommandHandlerFunc) gomabot.CommandHandlerFunc {
	return func(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
		err := fn(ctx, client, evt)
		if err == nil {
			return err
		}

		r := fmt.Sprintf("Error: %s", err.Error())
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := exitError.Stderr
			if err.Error() == "signal: killed" {
				stderr = []byte("command timed out")
			}

			if len(stderr) > 0 {
				r = fmt.Sprintf("Error: %s\n```\n%s\n```", err.Error(), stderr)
			}
		}

		return nb.SendMarkdownReply(ctx, client, evt, []byte(r))
	}
}
