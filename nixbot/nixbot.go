package nixbot

import (
	"bytes"
	"context"
	"fmt"

	"github.com/eyJhb/gomabot/gomabot"
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

	md goldmark.Markdown
}

func (nb *NixBot) Run(ctx context.Context) {
	nb.md = goldmark.New()
	// fmt.Println(nb.CommandHandlerSearchOption(ctx))
	// os.Exit(0)
	// return

	nb.Bot.AddEventHandler("^!ping", nb.CommandHandlerPing)
	nb.Bot.AddEventHandler("^!echo", nb.CommandHandlerEcho)
	nb.Bot.AddEventHandler("^!wiki (?P<search>.+)", nb.CommandHandlerSearchWiki)
	nb.Bot.AddEventHandler("^!options (?P<search>.+)", nb.CommandHandlerSearchOptions)
	nb.Bot.AddEventHandler("^!option (?P<search>.+)", nb.CommandHandlerSearchOption)
	nb.Bot.AddEventHandler("^!packages? (?P<search>.+)", nb.CommandHandlerSearchPackages)
}

func (nb *NixBot) vars(ctx context.Context) map[string]string {
	v := ctx.Value("matrixbot-vars")
	if mapv, ok := v.(map[string]string); ok {
		return mapv
	}

	return make(map[string]string)
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
