package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	gobot "github.com/eyJhb/gomabot/gomabot"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"maunium.net/go/mautrix/id"
)

var config_path = flag.String("config", "", "path to configuration file")

type ConfigCommandHandler struct {
	Pattern string
	Script  string
}

type config struct {
	Homeserver string
	PickleKey  string

	// Authentication UserID + AccessToken
	UserID      string
	AccessToken string

	// Authentication Username + Password
	Username string
	Password string

	StateDir string

	// Handlers []ConfigCommandHandler
	Handlers map[string]string
}

func main() {
	flag.Parse()
	if *config_path == "" {
		flag.Usage()
		os.Exit(1)
	}

	v := viper.NewWithOptions(viper.KeyDelimiter("::"))

	// set default shit, otherwise env won't work
	v.SetDefault("password", "")
	v.SetDefault("picklekey", "")

	v.SetConfigFile(*config_path)
	v.SetEnvPrefix("MATRIX_BOT")
	v.AutomaticEnv()
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config into struct")
		return
	}

	var conf config
	err = v.Unmarshal(&conf)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to decode config into struct")
		return
	}

	fmt.Println(run(conf))
}

func run(conf config) error {
	ctx := context.Background()

	botOpts := gobot.MatrixBotOpts{
		Homeserver: conf.Homeserver,
		PickleKey:  []byte(conf.PickleKey),

		UserID:      id.UserID(conf.UserID),
		AccessToken: conf.AccessToken,

		Username: conf.Username,
		Password: conf.Password,

		Database: fmt.Sprintf("%s/%s", conf.StateDir, "mautrix-database.db"),
	}

	// add handlers
	for handlerPattern, handlerScript := range conf.Handlers {
		botOpts.Handlers = append(botOpts.Handlers, gobot.CommandHandler{
			Pattern:         *regexp.MustCompile(fmt.Sprintf("^%s", handlerPattern)),
			Handler:         HandlerScript(handlerScript),
			OriginalPattern: handlerPattern,
		})
	}

	bot, err := gobot.NewMatrixBot(ctx, botOpts)
	if err != nil {
		return err
	}

	// start bot
	bot.Start(ctx)
	log.Info().Msg("started bot")

	// stopping bot
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Info().Msg("stopping bot")
	err = bot.Stop(ctx)
	if err != nil {
		return err
	}

	return nil
}

func HandlerScript(script string) func(ctx context.Context, sender id.UserID, room id.RoomID, message string) (string, error) {
	type scriptArgs struct {
		SenderID string
		RoomID   string
		Message  string
	}

	return func(ctx context.Context, sender id.UserID, room id.RoomID, message string) (string, error) {
		// marshal
		jsonScriptArgs, err := json.Marshal(scriptArgs{SenderID: string(sender), RoomID: room.String(), Message: message})
		if err != nil {
			return "", err
		}

		cmd := exec.CommandContext(ctx, script, string(jsonScriptArgs))

		// stderr
		var outb, errb bytes.Buffer
		cmd.Stderr = &errb
		cmd.Stdout = &outb

		// setup env
		env := os.Environ()
		env = append(env, fmt.Sprintf("USERID=%s", string(sender)))
		env = append(env, fmt.Sprintf("ROOMID=%s", string(room)))
		env = append(env, fmt.Sprintf("MESSAGE=%s", string(message)))

		message_split := strings.SplitN(message, " ", 2)

		if len(message_split) > 1 {
			env = append(env, fmt.Sprintf("MESSAGE_STRIP=%s", message_split[1]))
		} else {
			env = append(env, "MESSAGE_STRIP=")
		}

		cmd.Env = env

		// execute command
		err = cmd.Run()
		if err != nil {
			log.Error().Str("stdout", outb.String()).Str("stderr", errb.String()).Msg("failed to run command")
			return "", err
		}

		return outb.String(), nil
	}
}

func HandlerTest(sender id.UserID, room id.RoomID, message string) (string, error) {
	return "This is a response!!", nil
}
