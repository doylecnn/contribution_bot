package chatbots

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/doylecnn/contribution_bot/stackdriverhook"
	"github.com/doylecnn/contribution_bot/storage"
	"github.com/gin-contrib/logger"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ChatBot is chat bot
type ChatBot struct {
	logwriter       *stackdriverhook.StackdriverLoggingWriter
	logger          zerolog.Logger
	botClient       *tgbotapi.BotAPI
	router          router
	projectID       string
	appID           string
	token           string
	adminID         int
	forwardToChatID int64
	domain          string
	port            string
	storage         storage.Storage
}

// NewChatBot return new chat bot
func NewChatBot(token, domain, appID, projectID, port string, adminID int) ChatBot {
	var logger zerolog.Logger
	sw, err := stackdriverhook.NewStackdriverLoggingWriter(projectID, "bot", map[string]string{"from": "bot"})
	if err != nil {
		logger = log.Logger
		logger.Error().Err(err).Msg("new NewStackdriverLoggingWriter failed")
	} else {
		logger = zerolog.New(sw).Level(zerolog.DebugLevel)
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		logger.Fatal().Err(err).Send()
	}
	bot.Debug = false
	logger.Info().Str("bot username", bot.Self.UserName).
		Int("bot id", bot.Self.ID).Msg("authorized success")

	s := storage.NewStorage(projectID)

	c := ChatBot{botClient: bot,
		router:    newRouter(),
		projectID: projectID,
		appID:     appID,
		token:     token,
		logger:    logger,
		logwriter: sw,
		domain:    domain,
		port:      port,
		adminID:   adminID,
		storage:   s,
	}
	settings, err := s.GetSettings(context.Background())
	if err != nil {
		logger.Warn().Err(err).Msg("need set settings")
	} else {
		c.forwardToChatID = settings.ForwardMessageToChatID
	}

	c.initCommands()

	return c
}

// Run run the bot
func (c ChatBot) Run() {
	var zerologger zerolog.Logger
	sw, err := stackdriverhook.NewStackdriverLoggingWriter(c.projectID, "web", map[string]string{"from": "web"})
	if err != nil {
		zerologger = log.Logger
		zerologger.Error().Err(err).Msg("new NewStackdriverLoggingWriter failed")
	} else {
		defer sw.Close()
		zerologger = zerolog.New(sw).Level(zerolog.DebugLevel)
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(logger.SetLogger(logger.Config{
		Logger: &zerologger,
		UTC:    true,
	}), gin.Recovery())

	updates := make(chan tgbotapi.Update, c.botClient.Buffer)
	r.POST("/"+c.token, func(c *gin.Context) {
		bytes, _ := ioutil.ReadAll(c.Request.Body)

		var update tgbotapi.Update
		json.Unmarshal(bytes, &update)

		updates <- update
	})

	r.GET("/cron/clearmessages", c.cleanmessages)

	for i := 0; i < 2; i++ {
		go c.messageHandlerWorker(updates)
	}

	if err = c.SetWebhook(); err != nil {
		c.logger.Error().Err(err).Msg("SetWebhook failed")
	}
	r.Run(fmt.Sprintf(":%s", c.port))
}

// Close close bot
func (c ChatBot) Close() {
	c.storage.Close()
	c.logwriter.Close()
}

// SetWebhook set webhook
func (c ChatBot) SetWebhook() (err error) {
	info, err := c.botClient.GetWebhookInfo()
	if err != nil {
		return
	}
	if info.LastErrorDate != 0 {
		c.logger.Info().Str("last error message", info.LastErrorMessage).Msg("Telegram callback failed")
	}
	if !info.IsSet() {
		var webhookConfig WebhookConfig
		var wc = tgbotapi.NewWebhook(fmt.Sprintf("https://%s/%s", c.domain, c.token))
		webhookConfig = WebhookConfig{WebhookConfig: wc}
		webhookConfig.MaxConnections = 20
		webhookConfig.AllowedUpdates = []string{"message", "callback_query"}
		var apiResp tgbotapi.APIResponse
		apiResp, err = c.setWebhook(webhookConfig)
		if err != nil {
			c.logger.Error().Err(err).Msg("SetWebhook failed")
			return
		}
		info := c.logger.Info().Int("errorcode", apiResp.ErrorCode).
			Str("description", apiResp.Description).
			Bool("ok", apiResp.Ok).
			RawJSON("result", apiResp.Result)
		if apiResp.Parameters != nil {
			info.Dict("parameters", zerolog.Dict().
				Int64("migrateToChatID", apiResp.Parameters.MigrateToChatID).
				Int("retryAfter", apiResp.Parameters.RetryAfter),
			)
		}
		info.Msg("set webhook success")
	}
	return
}

func (c ChatBot) messageHandlerWorker(updates chan tgbotapi.Update) {
	for update := range updates {
		callbackQuery := update.CallbackQuery
		message := update.Message
		if (message != nil &&
			(message.From.IsBot ||
				message.LeftChatMember != nil ||
				message.NewChatMembers != nil)) ||
			(callbackQuery != nil &&
				callbackQuery.From.IsBot) {
			continue
		}
		if callbackQuery != nil {
			c.handleCallbackQuery(callbackQuery)
		} else if message != nil {
			if message.IsCommand() {
				err := c.router.run(c, message)
				if err != nil {
					c.logger.Error().Err(err).Send()
				}
			} else {
				if message.From.ID == c.adminID && message.ReplyToMessage != nil && message.ReplyToMessage.From.IsBot {
					if strings.HasPrefix(message.ReplyToMessage.Text, "change") {
						settings, _ := c.storage.GetSettings(context.Background())
						switch message.ReplyToMessage.Text {
						case "change welcome words:":
							settings.WelcomeWords = message.Text
							break
						case "change bot info:":
							settings.BotInfo = message.Text
							break
						case "change thanks words:":
							settings.Thanks = message.Text
							break
						case "change forward to chat id:":
							chatID, err := strconv.ParseInt(message.Text, 10, 64)
							if err == nil {
								settings.ForwardMessageToChatID = chatID
								c.forwardToChatID = settings.ForwardMessageToChatID
							} else {
								c.logger.Error().Err(err).Send()
							}
							break
						}
						err := c.storage.SaveSettings(context.Background(), settings)
						var replyText string
						if err != nil {
							c.logger.Error().Err(err).Send()
							replyText = "update failed\n error:" + err.Error() + "\n" + settings.String()
						} else {
							replyText = "update success\n" + settings.String()
						}
						c.botClient.Send(tgbotapi.MessageConfig{
							BaseChat: tgbotapi.BaseChat{
								ChatID:      message.Chat.ID,
								ReplyMarkup: settingsMarkup(),
							},
							Text: replyText,
						})
						continue
					}
				}
				if message.Chat.IsPrivate() {
					if c.forwardToChatID != 0 {
						if err := c.forward(message); err != nil {
							c.logger.Error().Err(err).Send()
						}
					}
				} else if message.ReplyToMessage != nil &&
					(message.Chat.IsGroup() ||
						message.Chat.IsSuperGroup()) {
					if err := c.reply(message); err != nil {
						c.logger.Error().Err(err).Send()
					}
				}
			}
		}
	}
}

func (c ChatBot) forward(message *tgbotapi.Message) error {
	var username string = message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName
		if len(username) == 0 {
			username = fmt.Sprintf("@%d", message.From.ID)
		}
	}
	msg := storage.Message{
		Username:  username,
		UserID:    message.From.ID,
		ChatID:    message.Chat.ID,
		MessageID: message.MessageID,
		Time:      message.Time(),
		Status:    "unread",
		ForwardID: 0,
	}
	docRef, err := c.storage.CreateNewMessage(context.Background(), msg)
	if err != nil {
		c.logger.Error().Err(err).Send()
		c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, "forward failed...try again?"))
		return err
	}
	settings, err := c.storage.GetSettings(context.Background())
	if err != nil {
		c.logger.Error().Err(err).Send()
	} else {
		c.forwardToChatID = settings.ForwardMessageToChatID
		sendm, err := c.botClient.Send(tgbotapi.NewForward(c.forwardToChatID, message.Chat.ID, message.MessageID))
		if err != nil {
			c.logger.Error().Err(err).
				Int64("forwardToChatID", c.forwardToChatID).
				Int64("originChatID", message.Chat.ID).
				Int("MessageID", message.MessageID).
				Send()
			return err
		}
		msg.ForwardID = sendm.MessageID
		msg.Status = "forward"
		err = c.storage.UpdateMessageStatus(context.Background(), docRef, msg)
		if err != nil {
			c.logger.Error().Err(err).Send()
		}
		_, err = c.botClient.Send(tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:           message.Chat.ID,
				ReplyToMessageID: message.MessageID,
			},
			Text: settings.Thanks,
		})
	}
	return err
}

func (c ChatBot) reply(message *tgbotapi.Message) (err error) {
	originmsg, err := c.storage.GetMessage(context.Background(), message.ReplyToMessage.MessageID)
	if err != nil {
		c.logger.Error().Err(err).Send()
		c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, "can not found source message"))
		return
	}
	_, err = c.botClient.Send(tgbotapi.NewMessage(originmsg.ChatID, message.Text))
	if err != nil {
		c.logger.Error().Err(err).Send()
		c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, "reply message failed"))
		return
	}
	return
}

// SetHelpInfo set help info
func (c ChatBot) setHelpInfo(helpInfo HelpInfo) {
	c.addCommandHandler("help", func(c ChatBot, message *tgbotapi.Message) (err error) {
		c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, helpInfo.String()))
		return
	})
}

func (c ChatBot) addCommandHandler(cmd string, handler CommandHandler) {
	if _, ok := c.router.commands[cmd]; ok {
		c.logger.Fatal().Err(errors.New("already exists handle func")).Send()
	} else {
		c.router.commands[cmd] = handler
	}
}

//HelpInfo help info
type HelpInfo struct {
	Description string
	Commands    []BotCommand
}

func (h HelpInfo) String() string {
	var cmdHelp []string = make([]string, len(h.Commands))
	for i, c := range h.Commands {
		cmdHelp[i] = fmt.Sprintf("%s %s", c.Command, c.Description)
	}
	return fmt.Sprintf("%s\n%s", h.Description, strings.Join(cmdHelp, "\n"))
}

func (c ChatBot) initCommands() {
	var commands []BotCommand

	// cmd start
	c.addCommandHandler("start", cmdStart)
	commands = append(commands, BotCommand{Command: "start", Description: "start use bot"})

	// cmd settings
	c.addCommandHandler("settings", cmdSettings)
	commands = append(commands, BotCommand{Command: "settings", Description: "admin change settings"})

	// cmd getChatID
	c.addCommandHandler("getchatid", cmdGetChatID)
	commands = append(commands, BotCommand{Command: "getchatid", Description: "get chat id"})

	var help HelpInfo
	settings, err := c.storage.GetSettings(context.Background())
	if err == nil {
		help.Description = settings.BotInfo
	}
	help.Commands = commands
	c.setHelpInfo(help)
	c.setMyCommands(commands)
}

func (c ChatBot) cleanmessages(ctx *gin.Context) {
	err := c.storage.DeleteOldForwardMessages(context.Background())
	if err != nil {
		c.logger.Error().Err(err).Send()
		ctx.JSON(200, "failed")
		ctx.Abort()
		return
	}
	ctx.JSON(200, "OK")
}
