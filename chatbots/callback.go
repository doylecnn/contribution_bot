package chatbots

import (
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (c ChatBot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	if query.From.ID != c.adminID {
		return
	}

	if !strings.HasPrefix(query.Data, "/change_") {
		return
	}

	var replyText string
	switch query.Data {
	case "/change_welcome_words":
		replyText = "change welcome words:"
		break
	case "/change_bot_info":
		replyText = "change bot info:"
		break
	case "/change_thanks":
		replyText = "change thanks words:"
		break
	case "/change_forward_to_chat_id":
		replyText = "change forward to chat id:"
		break
	case "/change_done":
		c.botClient.DeleteMessage(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
		c.botClient.AnswerCallbackQuery(tgbotapi.NewCallback(query.ID, "done"))
		return
	}
	c.botClient.DeleteMessage(tgbotapi.NewDeleteMessage(query.Message.Chat.ID, query.Message.MessageID))
	_, err := c.botClient.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      int64(query.From.ID),
			ReplyMarkup: tgbotapi.ForceReply{ForceReply: true, Selective: true},
		},
		Text: replyText,
	})
	if err != nil {
		c.logger.Error().Err(err).Send()
	}
	c.botClient.AnswerCallbackQuery(tgbotapi.NewCallback(query.ID, "update request received"))
}
