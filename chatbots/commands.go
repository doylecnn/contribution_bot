package chatbots

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdStart(c ChatBot, message *tgbotapi.Message) (err error) {
	settings, err := c.storage.GetSettings(context.Background())
	if err != nil {
		_, err = c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, "请先使用 /settings 修改设置"))
		return
	}
	_, err = c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, settings.BotInfo))
	if err != nil {
		return
	}
	_, err = c.botClient.Send(tgbotapi.NewMessage(message.Chat.ID, settings.WelcomeWords))
	return
}

func cmdSettings(c ChatBot, message *tgbotapi.Message) (err error) {
	if message.From.ID != c.adminID {
		return
	}
	settings, _ := c.storage.GetSettings(context.Background())
	_, err = c.botClient.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      message.Chat.ID,
			ReplyMarkup: settingsMarkup(),
		},
		Text: "change settings\n" + settings.String(),
	})
	return
}

func settingsMarkup() (replyMarkup tgbotapi.InlineKeyboardMarkup) {
	changeWelcomeWordsBtn := tgbotapi.NewInlineKeyboardButtonData("change welcome words", "/change_welcome_words")
	changeBotInfoBtn := tgbotapi.NewInlineKeyboardButtonData("change bot info", "/change_bot_info")
	changeThanksBtn := tgbotapi.NewInlineKeyboardButtonData("change thanks words", "/change_thanks")
	changeForwardToChatIDBtn := tgbotapi.NewInlineKeyboardButtonData("change forward to chat id", "/change_forward_to_chat_id")
	settingsDoneBtn := tgbotapi.NewInlineKeyboardButtonData("done", "/change_done")
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(changeWelcomeWordsBtn),
		tgbotapi.NewInlineKeyboardRow(changeBotInfoBtn),
		tgbotapi.NewInlineKeyboardRow(changeThanksBtn),
		tgbotapi.NewInlineKeyboardRow(changeForwardToChatIDBtn),
		tgbotapi.NewInlineKeyboardRow(settingsDoneBtn),
	)
}

func cmdGetChatID(c ChatBot, message *tgbotapi.Message) (err error) {
	if message.From.ID != c.adminID {
		return
	}
	_, err = c.botClient.Send(tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID: message.Chat.ID,
		},
		Text: fmt.Sprintf("%d", message.Chat.ID),
	})
	return
}
