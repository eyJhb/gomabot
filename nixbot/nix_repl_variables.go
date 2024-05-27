package nixbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
)

var (
	ErrReplVariableEmptyKeyOrExpr = errors.New("key or expr cannot be empty")
)

func (nb *NixBot) CommandHandlerAddReplVariable(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	key := strings.TrimSpace(vars["key"])
	expr := strings.TrimSpace(vars["expr"])

	if key == "" || expr == "" {
		return ErrReplVariableEmptyKeyOrExpr
	}

	err := nb.AddNixReplVariable(key, expr)
	if err != nil {
		return err
	}

	return nb.SendTextReply(ctx, client, evt, []byte(fmt.Sprintf("Defined key %s", key)))
}

func (nb *NixBot) CommandHandlerRemoveReplVariable(ctx context.Context, client *mautrix.Client, evt *event.Event) error {
	vars := nb.vars(ctx)
	key := strings.TrimSpace(vars["key"])

	if key == "" {
		return ErrReplVariableEmptyKeyOrExpr
	}

	err := nb.RemoveNixReplVariable(key)
	if err != nil {
		return err
	}

	return nb.SendTextReply(ctx, client, evt, []byte(fmt.Sprintf("Undefined key %s", key)))
}

func (nb *NixBot) LoadNixReplVariablesFile() error {
	nb.ReplFileLock.Lock()
	defer nb.ReplFileLock.Unlock()

	nb.ReplVariables = make(map[string]string)

	f, err := os.Open(nb.ReplFilePath)
	if err != nil {
		return err
	}

	return json.NewDecoder(f).Decode(&nb.ReplVariables)
}

func (nb *NixBot) AddNixReplVariable(key, val string) error {
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

func (nb *NixBot) RemoveNixReplVariable(key string) error {
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
