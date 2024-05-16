package nixbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const (
	URLNixWiki = "https://wiki.nixos.org"
)

func (nb *NixBot) CommandHandlerSearchWiki(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/w/api.php", URLNixWiki), nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("action", "query")
	q.Add("list", "search")
	q.Add("srlimit", "10")
	q.Add("srprop", "sectiontitle|snippet")
	q.Add("srinfo", "")
	q.Add("format", "json")
	q.Add("srsearch", vars["search"])
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	type MediaWikiQueryRes struct {
		Batchcomplete string `json:"batchcomplete"`
		Query         struct {
			Search []struct {
				Ns      int    `json:"ns"`
				Title   string `json:"title"`
				Pageid  int    `json:"pageid"`
				Snippet string `json:"snippet"`
			} `json:"search"`
		} `json:"query"`
	}

	var results MediaWikiQueryRes
	err = json.NewDecoder(resp.Body).Decode(&results)
	if err != nil {
		return err
	}

	tmplText := `
{{- range $v := .Query.Search}}
- [{{$v.Title}}](https://wiki.nixos.org/wiki/?curid={{$v.Pageid}})
{{- end -}}
	`
	tmpl, err := template.New("nixwiki").Parse(tmplText)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, results)
	if err != nil {
		return err
	}

	return nb.MakeMarkdownReply(ctx, client, evt, buf.Bytes())
}
