package main

import (
	"log"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if strings.TrimSpace(token) == "" {
		log.Fatal("TELEGRAM_TOKEN env variable is required")
	}

	statePath := os.Getenv("STATE_FILE")
	if strings.TrimSpace(statePath) == "" {
		statePath = "data/state.json"
	}

	store := newFileStore(statePath)
	manager, err := newConversationManager(store)
	if err != nil {
		log.Fatalf("cannot load state: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("cannot start bot: %v", err)
	}
	bot.Debug = os.Getenv("BOT_DEBUG") == "1"
	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updateConfig.AllowedUpdates = []string{"message"}

	updates := bot.GetUpdatesChan(updateConfig)
	for update := range updates {
		if update.Message == nil {
			continue
		}
		chatID := update.Message.Chat.ID

		var resp response
		if update.Message.IsCommand() {
			resp = manager.handleCommand(chatID, update.Message.Command())
		} else {
			resp = manager.handleMessage(chatID, update.Message.Text)
		}

		msg := tgbotapi.NewMessage(chatID, resp.Text)
		if resp.WithKeyboard {
			msg.ReplyMarkup = buildMainKeyboard()
		} else if resp.RemoveKeyboard {
			msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
		}
		msg.ParseMode = tgbotapi.ModeMarkdown

		if _, err := bot.Send(msg); err != nil {
			log.Printf("failed to send message to %d: %v", chatID, err)
			time.Sleep(time.Second)
		}
	}
}

func buildMainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{
		{
			tgbotapi.NewKeyboardButton(mainKeyboardOptions[0]),
			tgbotapi.NewKeyboardButton(mainKeyboardOptions[1]),
		},
		{
			tgbotapi.NewKeyboardButton(mainKeyboardOptions[2]),
			tgbotapi.NewKeyboardButton(mainKeyboardOptions[3]),
		},
		{
			tgbotapi.NewKeyboardButton(mainKeyboardOptions[4]),
		},
	}

	keyboard := tgbotapi.NewReplyKeyboard(rows...)
	keyboard.ResizeKeyboard = true
	keyboard.OneTimeKeyboard = false
	return keyboard
}
