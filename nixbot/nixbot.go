package nixbot

import (
	"context"
	"sync"
	"time"

	"github.com/eyJhb/gomabot/gomabot"
	"github.com/rs/zerolog/log"
	"github.com/yuin/goldmark"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

type NixBot struct {
	Bot *gomabot.MatrixBot

	// Message limits
	MaxMessageLength uint16

	// repl
	ReplFilePath  string
	ReplFileLock  sync.RWMutex
	ReplVariables map[string]string

	// options
	NixOptions           map[string]map[string]NixOption
	NixOptionLastUpdated map[string]time.Time

	// admins
	InvitePrivileges       []string
	ReplVariablePrivileges []string

	// markdown
	md goldmark.Markdown
}

func (nb *NixBot) Run(ctx context.Context) {
	nb.md = goldmark.New()

	nb.NixOptions = make(map[string]map[string]NixOption)
	nb.NixOptionLastUpdated = make(map[string]time.Time)

	if err := nb.LoadNixReplVariablesFile(); err != nil {
		log.Panic().Err(err).Msg("unable to load nix repl file")
	}

	mwr := nb.MiddlewareResponse
	mwt := nb.MiddlewareTimeout
	mwa := nb.MiddlewareAdmin

	nb.Bot.AddEventHandler("^!ping", mwr(nb.CommandHandlerPing))
	nb.Bot.AddEventHandler("^!echo", mwr(nb.CommandHandlerEcho))
	nb.Bot.AddEventHandler("^!wiki (?P<search>.+)", mwr(mwt(5*time.Second, nb.CommandHandlerSearchWiki)))
	nb.Bot.AddEventHandler("^!(?P<hm>hm)?options (?P<search>.+)", mwr(mwt(30*time.Second, nb.CommandHandlerSearchOptions)))
	nb.Bot.AddEventHandler("^!(?P<hm>hm)?option (?P<search>.+)", mwr(mwt(30*time.Second, nb.CommandHandlerSearchOption)))
	nb.Bot.AddEventHandler("^!packages? (?P<search>.+)", mwr(mwt(30*time.Second, nb.CommandHandlerSearchPackages)))

	// repl
	// https://spec.matrix.org/legacy/appendices#user-identifiers
	// nb.Bot.AddEventHandler(`(?s)^,\s*(?P<key>[A-z0-9_]+)\s*(\<(?P<user>@[a-z0-9-_\.]+:[^"]+))?$`, mwr(nb.CommandHandlerReplSimple))
	// nb.Bot.AddEventHandlerFormattedBody(`(?s)^,\s*(?P<key>[A-z0-9_ ]+)\s*(\<.*(?P<user>@[a-z0-9-_\.]+:[^"]+).*\>)?$`, mwr(nb.CommandHandlerReplSimple))
	nb.Bot.AddEventHandlerFormattedBody(`(?s)^,\s*(?P<key>[A-z0-9_ ]+)\s*(\<.*\/(?P<user>@[a-z0-9-_\.]+:[^"]+).*\>)?$`, mwr(nb.CommandHandlerReplSimple))

	nb.Bot.AddEventHandler(`(?s)^,\s*(?P<key>[A-z0-9_]+)\s*=(?P<expr>.+)`, mwr(mwa(nb.ReplVariablePrivileges, nb.CommandHandlerAddReplVariable)))
	nb.Bot.AddEventHandler(`(?s)^,\s*(?P<key>[A-z0-9_]+)\s*=`, mwr(mwa(nb.ReplVariablePrivileges, nb.CommandHandlerRemoveReplVariable)))
	nb.Bot.AddEventHandler(`(?s)^,\s*(?P<strict>:p)?(?P<expr>.+)`, mwr(mwt(5*time.Second, nb.CommandHandlerRepl)))
	// nb.Bot.AddEventHandler("(?s)^(?P<strict>eval)-?(?P<raw>raw)?.*```nix(?P<expr>.*)```", mwr(mwt(10*time.Second, nb.CommandHandlerRepl)))
	nb.Bot.AddEventHandler("(?s)^(?P<strict>eval)-?(?P<raw>raw)?(?P<package>package)?.*```nix(?P<expr>.*)```", mwr(mwt(10*time.Second, nb.CommandHandlerRepl)))

	nb.Bot.AddEventHandler(`^!help\s*$`, mwr(nb.CommandHandlerHelp))

	// setup who can invite the bot to rooms
	nb.Bot.RoomjoinHandler = mwa(nb.InvitePrivileges, func(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
		return nil
	})
}
