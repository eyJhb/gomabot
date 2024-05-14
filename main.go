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
	"sort"
	"strings"
	"syscall"

	"github.com/eyJhb/gomabot/gomabot"
	gobot "github.com/eyJhb/gomabot/gomabot"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"maunium.net/go/mautrix/id"
)

var config_path = flag.String("config", "", "path to configuration file")

type ConfigCommandHandler struct {
	Pattern string
	Script  string
}

type config struct {
	Homeserver string `yaml:"Homeserver"`
	PickleKey  string `yaml:"PickleKey"`

	// Authentication UserID + AccessToken
	UserID      string `yaml:"UserID"`
	AccessToken string `yaml:"AccessToken"`

	// Authentication Username + Password
	Username string `yaml:"Username"`
	Password string `yaml:"Password"`

	StateDir string `yaml:"StateDir"`

	ScriptHandlers    map[string]string `yaml:"ScriptHandlers"`
	ScriptJoinHandler string            `yaml:"ScriptJoinHandler"`
}

func main() {
	flag.Parse()
	if *config_path == "" {
		flag.Usage()
		os.Exit(1)
	}

	configBytes, err := os.ReadFile(*config_path)
	if err != nil {
		panic(err)
	}

	var conf config
	err = yaml.Unmarshal(configBytes, &conf)
	if err != nil {
		panic(err)
	}

	ENV_PREFIX := "MATRIX_BOT_"
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "PICKLEKEY")); v != "" {
		conf.PickleKey = v
	}
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "USERNAME")); v != "" {
		conf.Username = v
	}
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "PASSWORD")); v != "" {
		conf.Password = v
	}
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "USERID")); v != "" {
		conf.UserID = v
	}
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "ACCESSTOKEN")); v != "" {
		conf.AccessToken = v
	}

	err = run(conf)
	if err != nil {
		log.Error().Err(err).Msg("failed to run bot")
		os.Exit(1)
	}
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

		Handlers: prepareScriptHandlers(conf.ScriptHandlers),
	}

	if conf.ScriptJoinHandler != "" {
		botOpts.RoomjoinHandler = HandlerScript(conf.ScriptJoinHandler)
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

func prepareScriptHandlers(scriptHandlers map[string]string) []gomabot.CommandHandler {
	//  put into a tempoarary slice
	var scriptPatterns []string
	for scriptPattern := range scriptHandlers {
		scriptPatterns = append(scriptPatterns, scriptPattern)

	}

	sort.Slice(scriptPatterns, func(i, j int) bool { return len(scriptPatterns[i]) > len(scriptPatterns[j]) })

	// convert scripthandlers to handlers
	var handlers []gomabot.CommandHandler
	for _, scriptPattern := range scriptPatterns {
		scriptPath := scriptHandlers[scriptPattern]

		handlers = append(handlers, gobot.CommandHandler{
			Pattern: *regexp.MustCompile(scriptPattern),
			Handler: HandlerScript(scriptPath),
		})
	}

	// sorted handlers by longest pattern
	return handlers
}

func HandlerScript(script string) gomabot.CommandHandlerFunc {
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
