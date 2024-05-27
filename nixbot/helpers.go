package nixbot

import (
	"bytes"
	"context"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (nb *NixBot) vars(ctx context.Context) map[string]string {
	v := ctx.Value("matrixbot-vars")
	if mapv, ok := v.(map[string]string); ok {
		return mapv
	}

	return make(map[string]string)
}

func (nb *NixBot) SendTextNoResults(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	_, err := client.SendText(ctx, evt.RoomID, string("No results"))
	return err
}

func (nb *NixBot) SendTextReply(ctx context.Context, client *mautrix.Client, evt *event.Event, msg []byte) error {
	if len(msg) > int(nb.MaxMessageLength) && nb.MaxMessageLength != 0 {
		msg = []byte("Message exceeded the max message length")
	}

	_, err := client.SendText(ctx, evt.RoomID, string(msg))
	return err
}

func (nb *NixBot) SendMarkdownReply(ctx context.Context, client *mautrix.Client, evt *event.Event, markdown_raw []byte) error {
	if len(markdown_raw) > int(nb.MaxMessageLength) && nb.MaxMessageLength != 0 {
		markdown_raw = []byte("Message exceeded the max message length")
	}

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

func (nb *NixBot) SendMarkdownReplySummary(ctx context.Context, client *mautrix.Client, evt *event.Event, markdown_raw []byte, summary_text string) error {
	if len(markdown_raw) > int(nb.MaxMessageLength) && nb.MaxMessageLength != 0 {
		markdown_raw = []byte("Message exceeded the max message length")
	}

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
