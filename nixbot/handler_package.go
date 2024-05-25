package nixbot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"sort"
	"text/template"

	"github.com/hbollon/go-edlib"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

const (
	NixSearchPackagesLimit = 10
)

type Package struct {
	Description string `json:"description"`
	Pname       string `json:"pname"`
	Version     string `json:"version"`
}

type PackageName struct {
	Name string
	Package
}

func (nb *NixBot) CommandHandlerSearchPackages(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	search := vars["search"]

	cmd := exec.CommandContext(ctx,
		"nix", "search",
		"-I nixpkgs=channel:nixos-unstable",
		"--json",
		"nixpkgs",
		search,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return errors.New(stderr.String())
	}

	packagesMap := make(map[string]Package)
	err = json.Unmarshal(stdout.Bytes(), &packagesMap)
	if err != nil {
		return err
	}

	// convert to slice
	var packages []PackageName
	for k, v := range packagesMap {
		packages = append(packages, PackageName{Name: k, Package: v})
	}

	// sort slices
	sort.Slice(packages, func(i, j int) bool {
		s1, _ := edlib.StringsSimilarity(search, packages[i].Name, edlib.Levenshtein)
		s2, _ := edlib.StringsSimilarity(search, packages[j].Name, edlib.Levenshtein)
		return s1 < s2
	})

	tmplText := `
{{- range $v := .}}
- {{slice $v.Name 28}} ({{$v.Version}}) - [NixOS Search](https://search.nixos.org/packages?channel=unstable&query={{slice $v.Name 28}})
{{- end -}}
`
	tmpl, err := template.New("nixpackages").Parse(tmplText)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, packages)
	if err != nil {
		return err
	}

	return nb.MakeMarkdownReply(ctx, client, evt, buf.Bytes())
}
