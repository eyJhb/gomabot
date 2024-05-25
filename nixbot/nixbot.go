package nixbot

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/eyJhb/gomabot/gomabot"
	"github.com/rs/zerolog/log"
	"github.com/yuin/goldmark"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

// "^!admin" = scripts.wrappers.ensureMod (scripts.reply "$USERID is indeed a moderator!");

// "^(?s).*${Username}.*eval.*class=\"language-nix\"" = scripts.wrappers.ensureTimeout 60 scripts.nixEvalHtml;

// # repl things
// "^, ?[A-z0-9_ ]+ ?=.+" = scripts.wrappers._ensureMutex nixreplPath scripts.nixEvalAddRepl;
// "^, ?[A-z0-9_ ]+ ?=$" = scripts.wrappers._ensureMutex nixreplPath scripts.nixEvalRemoveRepl;
// "^, ?[A-z0-9 _]+$" = scripts.wrappers.ensureNoQuotes (scripts.wrappers.formatOutput (scripts.wrappers.ensureTimeout 60 scripts.nixEval));
// "^," = scripts.wrappers.formatOutput (scripts.wrappers.ensureTimeout 60 scripts.nixEval);

// "^!repl_show" = scripts.wrappers.formatOutput scripts.nixEvalShow;

type NixBot struct {
	Bot *gomabot.MatrixBot

	// repl
	ReplFilePath  string
	ReplFileLock  sync.RWMutex
	ReplVariables map[string]string

	// options
	NixOptions           map[string]NixOption
	NixOptionLastUpdated time.Time

	md goldmark.Markdown
}

func (nb *NixBot) Run(ctx context.Context) {
	nb.md = goldmark.New()
	// fmt.Println(nb.CommandHandlerSearchOption(ctx))
	// os.Exit(0)
	// return

	if err := nb.LoadNixReplFile(); err != nil {
		log.Panic().Err(err).Msg("unable to load nix repl file")
	}

	nb.Bot.AddEventHandler("^!ping", nb.CommandHandlerPing)
	nb.Bot.AddEventHandler("^!echo", nb.CommandHandlerEcho)
	nb.Bot.AddEventHandler("^!wiki (?P<search>.+)", nb.CommandHandlerSearchWiki)
	nb.Bot.AddEventHandler("^!options (?P<search>.+)", nb.CommandHandlerSearchOptions)
	nb.Bot.AddEventHandler("^!option (?P<search>.+)", nb.CommandHandlerSearchOption)
	nb.Bot.AddEventHandler("^!packages? (?P<search>.+)", nb.CommandHandlerSearchPackages)

	// repl
	nb.Bot.AddEventHandler("(?ms)^, ?(?P<key>.+)=(?P<expr>.+)", nb.CommandHandlerAddRepl)
	nb.Bot.AddEventHandler("(?ms)^, ?(?P<key>.+)=", nb.CommandHandlerRemoveRepl)
	nb.Bot.AddEventHandler("(?ms)^,(?P<strict>:p)?(?P<expr>.+)", nb.CommandHandlerRepl)
	nb.Bot.AddEventHandler("(?ms)^.*(?P<strict>eval).*```nix(?P<expr>.*)```", nb.CommandHandlerRepl)
	nb.Bot.AddEventHandler("(?ms)^.*(?P<strict>eval)-?(?P<raw>raw).*```nix(?P<expr>.*)```", nb.CommandHandlerRepl)
	// nb.Bot.AddEventHandler("(?ms)^.*{?P<strict>eval).*```nix(?P<expr>.*)```", nb.CommandHandlerRepl)
}

func (nb *NixBot) vars(ctx context.Context) map[string]string {
	v := ctx.Value("matrixbot-vars")
	if mapv, ok := v.(map[string]string); ok {
		return mapv
	}

	return make(map[string]string)
}

func (nb *NixBot) MakeReply(ctx context.Context, client *mautrix.Client, evt *event.Event, msg []byte) error {
	_, err := client.SendText(ctx, evt.RoomID, string(msg))
	return err
}

func (nb *NixBot) MakeMarkdownReply(ctx context.Context, client *mautrix.Client, evt *event.Event, markdown_raw []byte) error {
	// convert to markdown
	var markdown bytes.Buffer
	err := nb.md.Convert(markdown_raw, &markdown)
	if err != nil {
		return err
	}

	// send message
	_, err = client.SendMessageEvent(ctx, evt.RoomID, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    string(markdown_raw),

		Format:        event.FormatHTML,
		FormattedBody: markdown.String(),
	})

	return err
}

func (nb *NixBot) MakeMarkdownReplySummary(ctx context.Context, client *mautrix.Client, evt *event.Event, markdown_raw []byte, summary_text string) error {
	// convert to markdown
	var markdown bytes.Buffer
	err := nb.md.Convert(markdown_raw, &markdown)
	if err != nil {
		return err
	}

	// send message
	_, err = client.SendMessageEvent(ctx, evt.RoomID, event.EventMessage, &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    string(markdown_raw),

		Format:        event.FormatHTML,
		FormattedBody: fmt.Sprintf("<details><summary>%s</summary>\n%s\n</details>", summary_text, markdown.String()),
	})

	return err
}
