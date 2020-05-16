package chatbots

import (
	"encoding/json"
	"net/url"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// BotCommand BotCommand
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

func (c ChatBot) setMyCommands(commands []BotCommand) (response tgbotapi.APIResponse, err error) {
	v := url.Values{}
	var data []byte
	if data, err = json.Marshal(commands); err == nil {
		v.Add("commands", string(data))
	} else {
		return
	}

	return c.botClient.MakeRequest("setMyCommands", v)
}

func (c ChatBot) getMyCommands() (commands []BotCommand, err error) {
	v := url.Values{}
	resp, err := c.botClient.MakeRequest("getMyCommands", v)
	if err != nil {
		return
	}

	err = json.Unmarshal(resp.Result, &commands)
	return
}

// WebhookConfig contains information about a SetWebhook request.
type WebhookConfig struct {
	tgbotapi.WebhookConfig
	AllowedUpdates []string
}

// SetWebhook sets a webhook.
//
// If this is set, GetUpdates will not get any data!
//
// If you do not have a legitimate TLS certificate, you need to include
// your self signed certificate with the config.
func (c ChatBot) setWebhook(config WebhookConfig) (tgbotapi.APIResponse, error) {
	if config.Certificate == nil {
		v := url.Values{}
		v.Add("url", config.URL.String())
		if config.MaxConnections != 0 {
			v.Add("max_connections", strconv.Itoa(config.MaxConnections))
		}
		if len(config.AllowedUpdates) != 0 {
			v["allowed_updates"] = config.AllowedUpdates
		}

		return c.botClient.MakeRequest("setWebhook", v)
	}

	params := make(map[string]string)
	params["url"] = config.URL.String()
	if config.MaxConnections != 0 {
		params["max_connections"] = strconv.Itoa(config.MaxConnections)
	}

	resp, err := c.botClient.UploadFile("setWebhook", params, "certificate", config.Certificate)
	if err != nil {
		return tgbotapi.APIResponse{}, err
	}

	return resp, nil
}

func (c ChatBot) deleteWebhook() (tgbotapi.APIResponse, error) {
	return c.botClient.MakeRequest("deleteWebhook", url.Values{})
}
