package nixbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

func (nb *NixBot) CommandHandlerAddRepl(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	key := strings.TrimSpace(vars["key"])
	expr := strings.TrimSpace(vars["expr"])

	if key == "" || expr == "" {
		nb.MakeReply(ctx, client, evt, []byte("key or expr cannot be empty"))
	}

	err := nb.AddNixRepl(key, expr)
	if err != nil {
		return err
	}

	return nb.MakeReply(ctx, client, evt, []byte(fmt.Sprintf("Defined %s", key)))
}

func (nb *NixBot) CommandHandlerRemoveRepl(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	key := strings.TrimSpace(vars["key"])

	if key == "" {
		nb.MakeReply(ctx, client, evt, []byte("key cannot be empty"))
	}

	err := nb.RemoveNixRepl(key)
	if err != nil {
		return err
	}

	return nb.MakeReply(ctx, client, evt, []byte(fmt.Sprintf("Undefined %s", key)))
}

func (nb *NixBot) CommandHandlerRepl(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	// setup vars
	vars := nb.vars(ctx)
	var_expr := vars["expr"]

	var is_strict bool
	if vars["strict"] != "" {
		is_strict = true
	}

	var is_raw bool
	if vars["raw"] != "" {
		is_raw = true
	}

	finalNixExpr := var_expr
	var err error
	if !is_raw {
		finalNixExpr, err = nb.NixReplGenerateExpr(var_expr)
		if err != nil {
			return err
		}
	}

	// setup cmd args
	cmd_args := []string{
		"-I", "nixpkgs=channel:nixos-unstable",
		// basic limiting options
		"--option", "cores", "0",
		"--option", "fsync-metadata", "false",
		"--option", "restrict-eval", "true",
		"--option", "sandbox", "true",
		"--option", "timeout", "3",
		"--option", "max-jobs", "0",
		"--option", "allow-import-from-derivation", "false",
		"--option", "allowed-uris", "'[]'",
		"--option", "show-trace", "false",
	}

	// should strict flag be added?
	if is_strict {
		cmd_args = append(cmd_args, "--strict")
	}

	// eval expr
	cmd_args = append(cmd_args, []string{"--eval", "--expr", finalNixExpr}...)

	// setup command
	cmd := exec.CommandContext(ctx,
		"nix-instantiate", cmd_args...,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		errRes := fmt.Sprintf("Error\n```\n%s\n```", stderr.String())
		return nb.MakeMarkdownReply(ctx, client, evt, []byte(errRes))
	}

	log.Error().Err(err).Msg("failed to format code")

	stdoutString := strings.TrimSpace(stdout.String())
	formattedStdout, err := nb.FormatNix(ctx, stdoutString)
	if err != nil {
		log.Error().Err(err).Msg("failed to format nix output")

		// fallback to default text
		formattedStdout = stdoutString
	}

	if strings.Count(formattedStdout, "\n") > 0 {
		markdownRes := fmt.Sprintf("```nix\n%s\n```", formattedStdout)
		return nb.MakeMarkdownReply(ctx, client, evt, []byte(markdownRes))
	}

	return nb.MakeReply(ctx, client, evt, []byte(formattedStdout))
}

func (nb *NixBot) NixReplGenerateExpr(expr string) (string, error) {
	nb.ReplFileLock.RLock()
	defer nb.ReplFileLock.RUnlock()

	default_overrideable_nix_variables := map[string]string{
		"_show": "x: if lib.isDerivation x then \"<derivation ${x.drvPath}>\" else x",
	}

	default_nix_variables := map[string]string{
		"pkgs": "import <nixpkgs> {}",
		"lib":  "pkgs.lib",
	}

	final_nix_variables := map[string]string{}

	maps.Copy(final_nix_variables, default_overrideable_nix_variables)
	maps.Copy(final_nix_variables, nb.ReplVariables)
	maps.Copy(final_nix_variables, default_nix_variables)

	// because of @rasmus:rend.al, you know what you did
	delete(final_nix_variables, "builtins")

	tmplText := `
let
{{- range $k, $v := .Variables }}
  {{ $k }} = {{ $v }};
{{- end }}
in _show ( {{ .Expr }} )
`
	tmpl, err := template.New("nixeval").Parse(tmplText)
	if err != nil {
		return "", err
	}

	type templateTmp struct {
		Variables map[string]string
		Expr      string
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateTmp{Variables: final_nix_variables, Expr: expr})
	if err != nil {
		return "", err
	}

	return buf.String(), nil

}

func (nb *NixBot) FormatNix(ctx context.Context, input string) (string, error) {
	// setup cmd
	cmd := exec.CommandContext(ctx, "nixfmt")

	// setup stdin
	var stdin bytes.Buffer
	stdin.Write([]byte(input))
	cmd.Stdin = &stdin

	// get stdout, stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}

// nix repl
func (nb *NixBot) LoadNixReplFile() error {
	nb.ReplFileLock.Lock()
	defer nb.ReplFileLock.Unlock()

	nb.ReplVariables = make(map[string]string)

	f, err := os.Open(nb.ReplFilePath)
	if err != nil {
		return err
	}

	return json.NewDecoder(f).Decode(&nb.ReplVariables)
}

func (nb *NixBot) AddNixRepl(key, val string) error {
	nb.ReplFileLock.Lock()
	defer nb.ReplFileLock.Unlock()

	// add to map
	nb.ReplVariables[key] = val

	// marshal
	newFileBytes, err := json.Marshal(nb.ReplVariables)
	if err != nil {
		return err
	}

	return os.WriteFile(nb.ReplFilePath, newFileBytes, 0666)
}

func (nb *NixBot) RemoveNixRepl(key string) error {
	nb.ReplFileLock.Lock()
	defer nb.ReplFileLock.Unlock()

	// delete from map
	delete(nb.ReplVariables, key)

	// marshal
	newFileBytes, err := json.Marshal(nb.ReplVariables)
	if err != nil {
		return err
	}

	return os.WriteFile(nb.ReplFilePath, newFileBytes, 0666)
}
