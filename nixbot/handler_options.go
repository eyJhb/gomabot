package nixbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"text/template"

	"github.com/hbollon/go-edlib"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const (
	NixSearchOptionsLimit = 10
)

type NixOption struct {
	Declarations []string `json:"declarations"`
	Default      struct {
		Type string `json:"_type"`
		Text string `json:"text"`
	} `json:"default"`
	Description string   `json:"description"`
	Loc         []string `json:"loc"`
	ReadOnly    bool     `json:"readOnly"`
	Type        string   `json:"type"`
}

type NixOptionName struct {
	Name string
	NixOption
}

func (nb *NixBot) FetchNixOptions(ctx context.Context) (map[string]NixOption, error) {
	cmd := exec.CommandContext(ctx,
		"nix", "build",
		"-I nixpkgs=channel:nixos-unstable",
		"--impure",
		"--no-allow-import-from-derivation",
		"--restrict-eval",
		"--sandbox",
		"--no-link",
		"--json",
		"--expr",
		`
	      with import <nixpkgs> {}; let
            eval = import (pkgs.path + "/nixos/lib/eval-config.nix") { modules = []; };
            opts = (nixosOptionsDoc { options = eval.options; }).optionsJSON;
	      in runCommandLocal "options.json" {inherit opts; } "cp $opts/share/doc/nixos/options.json $out"
		`,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, errors.New(stderr.String())
	}

	type NixBuildResult []struct {
		Outputs struct {
			Out string `json:"out"`
		} `json:"outputs"`
	}

	var buildResults NixBuildResult
	err = json.Unmarshal(stdout.Bytes(), &buildResults)
	if err != nil {
		return nil, err
	}

	if len(buildResults) < 1 {
		return nil, errors.New("failed to build options.json")
	}

	optionsJSONFile := buildResults[0].Outputs.Out

	// open file
	f, err := os.Open(optionsJSONFile)
	if err != nil {
		return nil, err
	}

	nixOptions := make(map[string]NixOption)
	err = json.NewDecoder(f).Decode(&nixOptions)
	if err != nil {
		return nil, err
	}

	return nixOptions, nil
}

func (nb *NixBot) CommandHandlerSearchOptions(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	search := vars["search"]

	nixOptions, err := nb.FetchNixOptions(ctx)
	if err != nil {
		return err
	}

	// filteredOptions := make(map[string]NixOption)
	var filteredOptions []NixOptionName
	for k, v := range nixOptions {
		if ok, _ := regexp.MatchString("(?i)"+search, k); !ok {
			continue
		}

		filteredOptions = append(filteredOptions, NixOptionName{Name: k, NixOption: v})

		if len(filteredOptions) == NixSearchOptionsLimit {
			break
		}
	}

	sort.Slice(filteredOptions, func(i, j int) bool { return filteredOptions[i].Name < filteredOptions[j].Name })

	tmplText := `
{{- range $v := .}}
- [{{html $v.Name}}](https://search.nixos.org/options?channel=unstable&query={{$v.Name}})
{{- end -}}
		`
	tmpl, err := template.New("nixoptions").Parse(tmplText)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, filteredOptions)
	if err != nil {
		return err
	}

	return nb.MakeMarkdownReply(ctx, client, evt, buf.Bytes())
}

func (nb *NixBot) CommandHandlerSearchOption(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	search := vars["search"]

	nixOptions, err := nb.FetchNixOptions(ctx)
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

	option, ok := nixOptions[res]
	if !ok {
		return errors.New("unable to find option")
	}

	optionName := NixOptionName{
		Name:      res,
		NixOption: option,
	}

	tmplText := `
**Name**: {{html .Name}}

**Description**: {{.NixOption.Description}}

**Type**: {{.NixOption.Type}}

**Default**:
` + "```nix" + `
{{.NixOption.Default.Text}}
` + "```" + `

**Example**:
` + "```nix" + `
{{.NixOption.Default.Text}}
` + "```" + `

**Declared in** [{{slice (index .NixOption.Declarations 0) 56}}](https://github.com/NixOS/nixpkgs/blob/nixos-unstable/{{slice (index .NixOption.Declarations 0) 56}})

**Options page** [NixOS Options](https://search.nixos.org/options?channel=unstable&query={{.Name}})
			`
	tmpl, err := template.New("nixoption").Parse(tmplText)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, optionName)
	if err != nil {
		return err
	}

	return nb.MakeMarkdownReplySummary(ctx, client, evt, buf.Bytes(), html.EscapeString(res))
}
