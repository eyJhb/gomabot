package nixbot

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"time"
)

const (
	NixSearchOptionsLimit = 10
)

type NixOption struct {
	Declarations []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"declarations"`
	Default struct {
		Type string `json:"_type"`
		Text string `json:"text"`
	} `json:"default"`
	Example struct {
		Type string `json:"_type"`
		Text string `json:"text"`
	} `json:"example"`
	Description string   `json:"description"`
	Loc         []string `json:"loc"`
	ReadOnly    bool     `json:"readOnly"`
	Type        string   `json:"type"`
}

type NixOptionName struct {
	Name string
	NixOption
}

func (nb *NixBot) FetchNixOSOptions(ctx context.Context) (map[string]NixOption, error) {
	cmd := exec.CommandContext(ctx,
		"nix", "build",
		"-I", "nixpkgs=channel:nixos-unstable",
		"--impure",
		"--no-allow-import-from-derivation",
		"--restrict-eval",
		"--sandbox",
		"--no-link",
		"--json",
		"--expr",
		`
	      with import <nixpkgs> {}; with lib; let
	        eval = import (pkgs.path + "/nixos/lib/eval-config.nix") { modules = []; };
	        opts = (nixosOptionsDoc {
                     options = eval.options;
                     transformOptions = opt: opt // {
                       # Clean up declaration sites to not refer to the NixOS source tree.
                       declarations =
                         map
                           (decl:
                             let subpath = removePrefix "/" (removePrefix (toString <nixpkgs>) (toString decl));
                             in { url = "https://github.com/NixOS/nixpkgs/blob/master/${subpath}"; name = "<nixpkgs/${subpath}>"; })
                           opt.declarations;
                     };

            }).optionsJSON;
	      in runCommandLocal "options.json" {inherit opts; } "cp $opts/share/doc/nixos/options.json $out"
		`,
	)

	return nb.FetchOptions(ctx, "nixos", cmd)
}

func (nb *NixBot) FetchHomeManagerOptions(ctx context.Context) (map[string]NixOption, error) {
	cmd := exec.CommandContext(ctx,
		"nix", "build",
		"-I", "nixpkgs=channel:nixos-unstable",
		"-I", "home-manager=flake:github:nix-community/home-manager",
		"--impure",
		"--no-allow-import-from-derivation",
		"--restrict-eval",
		"--sandbox",
		"--no-link",
		"--json",
		"--expr",
		`
	    with import <home-manager> {};
	    let
	      pkgs = import <nixpkgs> {};
	      optionsJSON = docs.json;
	    in pkgs.runCommandLocal "options.json" {inherit optionsJSON; } "cp $optionsJSON/share/doc/home-manager/options.json $out"
		`,
	)
	// in pkgs.runCommandLocal "options.json" {inherit optionsJSON; } "${pkgs.jq}/bin/jq '. | to_entries | map(.value.declarations |= map(.url)) | from_entries' $optionsJSON/share/doc/home-manager/options.json > $out"

	return nb.FetchOptions(ctx, "home-manager", cmd)
}

func (nb *NixBot) FetchOptions(ctx context.Context, optionsKey string, cmd *exec.Cmd) (map[string]NixOption, error) {
	if nb.NixOptions[optionsKey] != nil && time.Now().Sub(nb.NixOptionLastUpdated[optionsKey]) < 30*time.Minute && !nb.NixOptionLastUpdated[optionsKey].IsZero() {
		return nb.NixOptions[optionsKey], nil
	}

	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	type NixBuildResult []struct {
		Outputs struct {
			Out string `json:"out"`
		} `json:"outputs"`
	}

	var buildResults NixBuildResult
	err = json.Unmarshal(stdout, &buildResults)
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

	nb.NixOptions[optionsKey] = nixOptions
	nb.NixOptionLastUpdated[optionsKey] = time.Now()

	return nixOptions, nil
}
