package chatbots

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// CommandHandler handle command
type CommandHandler func(c ChatBot, message *tgbotapi.Message) (err error)

type router struct {
	commands map[string]CommandHandler
}

func newRouter() router {
	r := router{}
	r.commands = make(map[string]CommandHandler)
	return r
}

func (r router) run(c ChatBot, message *tgbotapi.Message) (err error) {
	if !message.IsCommand() {
		return
	}
	command := message.Command()
	if cmd, ok := r.commands[command]; ok {
		e := cmd(c, message)
		if e != nil {
			err = fmt.Errorf("error occurred when running cmd: %s: error is: %w", command, e)
			return
		}
		return
	}
	err = fmt.Errorf("no HandleFunc for command /%s", command)
	return
}
