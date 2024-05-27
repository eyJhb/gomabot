package nixbot

import (
	"bytes"
	"context"
	"html"
	"sort"
	"text/template"

	"github.com/hbollon/go-edlib"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

var (
	tmplNixOptions = template.Must(template.New("nix-options").Parse(`
{{- range $v := .}}
- [{{html $v.Name}}](https://search.nixos.org/options?channel=unstable&query={{$v.Name}})
{{- end -}}
`))

	tmplNixOption = template.Must(template.New("nix-option").Parse(`
**Name**: {{html .Name}}

**Description**: {{.NixOption.Description}}

**Type**: {{.NixOption.Type}}

**Default**:
` + "```nix" + `
{{.NixOption.Default.Text}}
` + "```" + `

**Example**:
` + "```nix" + `
{{.NixOption.Example.Text}}
` + "```" + `

**Declared in** [{{ (index .NixOption.Declarations 0).Name }}]({{ (index .NixOption.Declarations 0).URL }})

			`))
	// **Options page** [NixOS Options](https://search.nixos.org/options?channel=unstable&query={{.Name}})
)

func (nb *NixBot) CommandHandlerSearchOptions(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	search := vars["search"]
	hm := vars["hm"]

	// fetch the correct set of options
	var err error
	var nixOptions map[string]NixOption
	if hm != "" {
		nixOptions, err = nb.FetchHomeManagerOptions(ctx)
	} else {
		nixOptions, err = nb.FetchNixOSOptions(ctx)
	}
	if err != nil {
		return err
	}

	// fuzzy search all the options
	var nixOptionKeys []string
	for k := range nixOptions {
		nixOptionKeys = append(nixOptionKeys, k)
	}

	res, err := edlib.FuzzySearchSet(search, nixOptionKeys, NixSearchOptionsLimit, edlib.Levenshtein)
	if err != nil {
		return err
	}

	var filteredOptions []NixOptionName
	for _, k := range res {
		if v, ok := nixOptions[k]; ok {
			filteredOptions = append(filteredOptions, NixOptionName{Name: k, NixOption: v})
		}

	}

	sort.Slice(filteredOptions, func(i, j int) bool { return filteredOptions[i].Name < filteredOptions[j].Name })

	// execute template
	var buf bytes.Buffer
	err = tmplNixOptions.Execute(&buf, filteredOptions)
	if err != nil {
		return err
	}

	return nb.SendMarkdownReply(ctx, client, evt, buf.Bytes())
}

func (nb *NixBot) CommandHandlerSearchOption(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	search := vars["search"]
	hm := vars["hm"]

	var err error
	var nixOptions map[string]NixOption
	if hm != "" {
		nixOptions, err = nb.FetchHomeManagerOptions(ctx)
	} else {
		nixOptions, err = nb.FetchNixOSOptions(ctx)
	}
	if err != nil {
		return err
	}

	// make a list of strings from map values
	var nixOptionKeys []string
	for k := range nixOptions {
		nixOptionKeys = append(nixOptionKeys, k)
	}

	res, err := edlib.FuzzySearch(search, nixOptionKeys, edlib.Levenshtein)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmplNixOption.Execute(&buf, NixOptionName{
		Name:      res,
		NixOption: nixOptions[res],
	})
	if err != nil {
		return err
	}

	return nb.SendMarkdownReplySummary(ctx, client, evt, buf.Bytes(), html.EscapeString(res))
}
