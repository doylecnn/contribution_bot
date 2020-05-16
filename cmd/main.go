package main

import (
	"errors"
	"os"
	"strconv"

	"github.com/doylecnn/contribution_bot/chatbots"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type env struct {
	Port       string
	BotToken   string
	BotAdminID int
	AppID      string
	Domain     string
	ProjectID  string
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	env := readEnv()

	bot := chatbots.NewChatBot(env.BotToken,
		env.Domain,
		env.AppID,
		env.ProjectID,
		env.Port,
		env.BotAdminID)
	defer bot.Close()

	bot.Run()
}

func readEnv() env {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
		log.Info().Str("port", port).Msg("set default port")
	}

	token := os.Getenv("BOT_TOKEN")

	botAdmin := os.Getenv("BOT_ADMIN")
	if len(botAdmin) == 0 {
		err := errors.New("not set env BOT_ADMIN")
		log.Logger.Fatal().Err(err).Send()
	}
	botAdminID, err := strconv.ParseInt(botAdmin, 10, 64)
	if err != nil {
		log.Logger.Fatal().Err(err).Send()
	}

	appID := os.Getenv("GAE_APPLICATION")
	if len(appID) == 0 {
		log.Logger.Fatal().Msg("no env var: GAE_APPLICATION")
	}
	appID = appID[2:]
	log.Logger.Info().Str("appID", appID).Send()

	projectID := os.Getenv("PROJECT_ID")
	if len(projectID) == 0 {
		log.Logger.Fatal().Msg("no env var: PROJECT_ID")
	}

	domain := os.Getenv("DOMAIN")
	if len(domain) == 0 {
		log.Logger.Fatal().Msg("no env var: DOMAIN")
	}

	return env{port, token, int(botAdminID), appID, domain, projectID}
}
