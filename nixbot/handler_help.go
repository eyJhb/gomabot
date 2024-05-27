package nixbot

import (
	"context"
	"html"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (nb *NixBot) CommandHandlerHelp(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	help := `
**NixOS Wiki**
- !wiki <search-term> - provides a list of links to wiki.nixos.org

**Searching Options**
- !options <fuzzy-search-term> - provides a list of NixOS options
- !option <fuzzy-search-term> - provides details about a single NixOS option
- !hmoptions <fuzzy-search-term> - provides a list of home-manager options
- !hmoption <fuzzy-search-term> - provides details about a single home-manager option

**Misc Commands**
- !ping - replies back with !pong and the time it took before the boot received the message
- !echo - echos what was typed back

**Evaluating Nix Expressions**
- , <nix-expr> - evaluates the nix-repl and returns the result
  - Expression can be prefixed with ` + "`" + ":p" + "`" + ` to enable strict mode, e.g.  ` + "`" + ", :p { a = { b = 1; }; }" + "`" + `
  - All expressions are passed through a ` + "`" + "_show" + "`" + ` function, which can be used to make the output pretty. The function can be overriden like any other variable
- , <key> = <nix-expr> - adds a variable that is available when evaluating nix expressions
- , <key> = - unsets a variable previously set

It is also possible to evaluate a codeblock, this is done by sending a message that starts with ` + "`" + "eval" + "`" + `, and then a nix codeblock.

~~~
eval

` + "```nix" + `
2+2
` + "```" + `
~~~

This will be evaluated the same way as if using ` + "`" + ", :p" + "`" + `.
However, it is also possible to evaluate an expression without the surrounding variables that the bot adds, or the ` + "`" + "_show" + "`" + ` function.
This is done as follows.


~~~
eval-raw

` + "```nix" + `
2+2
` + "```" + `
~~~
`

	return nb.SendMarkdownReply(ctx, client, evt, []byte(strings.TrimSpace(html.EscapeString(help))))
}
