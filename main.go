package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	gobot "github.com/eyJhb/gomabot/gomabot"
	"github.com/eyJhb/gomabot/nixbot"
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

	// admins
	Admins     []string `yaml:"Admins"`
	ReplAdmins []string `yaml:"ReplAdmins"`

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
	if v := os.Getenv(fmt.Sprintf("%s%s", ENV_PREFIX, "ADMINS")); v != "" {
		conf.Admins = strings.Split(v, ",")
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
	}

	bot, err := gobot.NewMatrixBot(ctx, botOpts)
	if err != nil {
		return err
	}

	bot.HandlerSendErrors = true

	nbot := nixbot.NixBot{
		Bot:              &bot,
		ReplFilePath:     fmt.Sprintf("%s/nixrepl.json", conf.StateDir),
		MaxMessageLength: 10000,

		InvitePrivileges:       conf.Admins,
		ReplVariablePrivileges: append(conf.ReplAdmins, conf.Admins...),
	}

	nbot.Run(ctx)

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
