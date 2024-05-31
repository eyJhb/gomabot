package gomabot

import (
	"context"
	"errors"
	"regexp"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/cryptohelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MatrixBot struct {
	Client *mautrix.Client

	Handlers          []CommandHandler
	HandlerSendErrors bool
	RoomjoinHandler   CommandHandlerFunc

	cryptoHelper  *cryptohelper.CryptoHelper
	waitGroup     *sync.WaitGroup
	ctxCancelFunc context.CancelFunc
}

type CommandHandlerFunc func(ctx context.Context, client *mautrix.Client, evt *event.Event) error

type CommandHandler struct {
	Pattern regexp.Regexp

	MatchFormattedBody bool

	Handler CommandHandlerFunc
}

type MatrixBotOpts struct {
	Homeserver string

	// if logging in with username/password
	Username string
	Password string

	// if logging in with userid and accesstoken
	UserID      id.UserID
	AccessToken string

	// database location, provide a .db here.
	// https://pkg.go.dev/maunium.net/go/mautrix/crypto/cryptohelper#NewCryptoHelper
	// stores all events, E2EE, etc. etc.
	Database  string
	PickleKey []byte

	// handlers
	Handlers []CommandHandler

	RoomjoinHandler CommandHandlerFunc
}

func NewMatrixBot(ctx context.Context, opts MatrixBotOpts) (MatrixBot, error) {
	// check if either userID + accessToken is set, or username + password is set
	if (opts.UserID == "" || opts.AccessToken == "") && (opts.Username == "" || opts.Password == "") {
		return MatrixBot{}, errors.New("either userid + accesstoken must be specified, or username + password")
	}

	var wg sync.WaitGroup
	mb := MatrixBot{
		waitGroup:       &wg,
		Handlers:        opts.Handlers,
		RoomjoinHandler: opts.RoomjoinHandler,
	}

	client, err := mautrix.NewClient(opts.Homeserver, opts.UserID, opts.AccessToken)
	if err != nil {
		return MatrixBot{}, err
	}
	mb.Client = client

	cryptoHelper, err := cryptohelper.NewCryptoHelper(client, opts.PickleKey, opts.Database)
	if err != nil {
		return MatrixBot{}, err
	}

	mb.cryptoHelper = cryptoHelper

	// if accesstoken and userid is empty, then login using username/password
	if opts.AccessToken == "" && opts.UserID == "" {
		cryptoHelper.LoginAs = &mautrix.ReqLogin{
			Type:       mautrix.AuthTypePassword,
			Identifier: mautrix.UserIdentifier{Type: mautrix.IdentifierTypeUser, User: opts.Username},
			Password:   opts.Password,
		}
	}
	// If you want to use multiple clients with the same DB, you should set a distinct database account ID for each one.
	err = cryptoHelper.Init(ctx)
	if err != nil {
		return MatrixBot{}, err
	}

	// Set the client crypto helper in order to automatically encrypt outgoing messages
	client.Crypto = cryptoHelper

	return mb, nil
}

func (mb *MatrixBot) Start(ctx context.Context) error {
	// get syncer
	syncer := mb.Client.Syncer.(*mautrix.DefaultSyncer)

	// setup new context for canceling
	newCtx, cancelFunc := context.WithCancel(ctx)
	mb.ctxCancelFunc = cancelFunc

	// start syncer
	mb.waitGroup.Add(1)
	go func() {
		err := mb.Client.SyncWithContext(newCtx)
		defer mb.waitGroup.Done()
		if err != nil && !errors.Is(err, context.Canceled) {
			panic(err)
		}
	}()

	// setup hooks
	syncer.OnEventType(event.EventMessage, mb.OnEventMessage)
	syncer.OnEventType(event.StateMember, mb.OnEventStateMember)

	return nil
}

func (mb *MatrixBot) Stop(ctx context.Context) error {
	// stop context
	mb.ctxCancelFunc()

	// ctx should be canceled before this
	mb.waitGroup.Wait()

	// close cryptohelper
	err := mb.cryptoHelper.Close()
	if err != nil {
		return err
	}

	return nil
}
