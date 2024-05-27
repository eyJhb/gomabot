package nixbot

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os/exec"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

var (
	tmplNixReplVariables = template.Must(template.New("packages").Parse(`
let
{{- range $k, $v := .Variables }}
  {{ $k }} = {{ $v }};
{{- end }}
in _show ( {{ .Expr }} )
`))

	fmtStrNixReplPackage = `let
  isDerivation = value: value.type or null == "derivation";
  _show = x: if isDerivation x then "<derivation ${x.drvPath}>" else x; 
in _show ( (import <nixpkgs> {}).pkgs.callPackage ( %s ) {} )`

	nix_repl_default_overrideable_nix_variables = map[string]string{
		"_show": "x: if lib.isDerivation x then \"<derivation ${x.drvPath}>\" else x",
	}

	nix_repl_default_nix_variables = map[string]string{
		"pkgs": "import <nixpkgs> {}",
		"lib":  "pkgs.lib",
	}
)

func (nb *NixBot) CommandHandlerReplSimple(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	var_key := vars["key"]
	var_user := vars["user"]

	expr := var_key
	if var_user != "" {
		expr = fmt.Sprintf("%s ''[%s](https://matrix.to/#/%s)''", var_key, var_user, var_user)

	}

	finalNixExpr, err := nb.NixReplGenerateExpr(expr)
	if err != nil {
		return err
	}

	stdout, err := nb.NixReplEvaluateExpr(ctx, finalNixExpr, true)
	if err != nil {
		return err
	}

	if len(stdout) > 2 && stdout[0] == '"' && stdout[len(stdout)-1] == '"' {
		stdout = stdout[1 : len(stdout)-1]
	}

	return nb.SendMarkdownReply(ctx, client, evt, []byte(stdout))
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

	var is_package bool
	if vars["package"] != "" {
		is_package = true
	}

	// setup expression
	finalNixExpr := var_expr
	var err error
	if is_package {
		finalNixExpr = fmt.Sprintf(fmtStrNixReplPackage, var_expr)
	} else if !is_raw {
		finalNixExpr, err = nb.NixReplGenerateExpr(var_expr)
		if err != nil {
			return err
		}
	}

	// evaluate nix expression
	stdout, err := nb.NixReplEvaluateExpr(ctx, finalNixExpr, is_strict)
	if err != nil {
		return err
	}

	// try to format the output as nix, if it fails then
	// fallback to the original stdout
	formattedStdout, err := nb.FormatNix(ctx, stdout)
	if err != nil {
		log.Error().Err(err).Msg("failed to format nix output")
		formattedStdout = stdout
	}

	// only turn into a markdown reply, if output contains newlines
	if strings.Contains(formattedStdout, "\n") {
		markdownRes := fmt.Sprintf("```nix\n%s\n```", formattedStdout)
		return nb.SendMarkdownReply(ctx, client, evt, []byte(markdownRes))
	}

	return nb.SendTextReply(ctx, client, evt, []byte(formattedStdout))
}

func (nb *NixBot) NixReplEvaluateExpr(ctx context.Context, expr string, is_strict bool) (string, error) {
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
	cmd_args = append(cmd_args, []string{"--eval", "--expr", expr}...)

	// setup command
	cmd := exec.CommandContext(ctx,
		"nix-instantiate", cmd_args...,
	)

	stdout, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stdout)), nil
}

func (nb *NixBot) NixReplGenerateExpr(expr string) (string, error) {
	nb.ReplFileLock.RLock()
	defer nb.ReplFileLock.RUnlock()

	final_nix_variables := map[string]string{}
	maps.Copy(final_nix_variables, nix_repl_default_overrideable_nix_variables)
	maps.Copy(final_nix_variables, nb.ReplVariables)
	maps.Copy(final_nix_variables, nix_repl_default_nix_variables)

	// because of @rasmus:rend.al, you know what you did
	delete(final_nix_variables, "builtins")

	type templateTmp struct {
		Variables map[string]string
		Expr      string
	}

	var buf bytes.Buffer
	err := tmplNixReplVariables.Execute(&buf, templateTmp{Variables: final_nix_variables, Expr: expr})
	if err != nil {
		return "", err
	}

	return buf.String(), nil

}

func (nb *NixBot) FormatNix(ctx context.Context, input string) (string, error) {
	cmd := exec.CommandContext(ctx, "nixfmt")

	// setup stdin
	var stdin bytes.Buffer
	stdin.Write([]byte(input))
	cmd.Stdin = &stdin

	stdout, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stdout)), nil
}
