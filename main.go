package main

import (
	"fmt"
	"log"
	"net/http"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"
	"strings"

	"github.com/m1guelpf/chatgpt-telegram/src/chatgpt"
	"github.com/m1guelpf/chatgpt-telegram/src/config"
	"github.com/m1guelpf/chatgpt-telegram/src/session"
	"github.com/m1guelpf/chatgpt-telegram/src/tgbot"
)

const LAVA_API_KEY = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJ1aWQiOiJiOWJjOWJmYi02MzhkLTdlZGMtYzgyOS1mYjUwYmQwMmIzOTUiLCJ0aWQiOiI4YmQ2YTdiYi1hZTRmLTE2NWItZGIyZS05MzdhMDc2NDJkZDcifQ.T-Jtx-SPHzwEr5lYLrkfQrpPi-I4DkEikSy_bmQB_Yk"

func main() {
	persistentConfig, err := config.LoadOrCreatePersistentConfig()
	if err != nil {
		log.Fatalf("Couldn't load config: %v", err)
	}

	if persistentConfig.OpenAISession == "" {
		token, err := session.GetSession()
		if err != nil {
			log.Fatalf("Couldn't get OpenAI session: %v", err)
		}
		if err = persistentConfig.SetSessionToken(token); err != nil {
			log.Fatalf("Couldn't save OpenAI session: %v", err)
		}
	}

	chatGPT := chatgpt.Init(persistentConfig)
	log.Println("Started ChatGPT")

	envConfig, err := config.LoadEnvConfig(".env")
	if err != nil {
		log.Fatalf("Couldn't load .env config: %v", err)
	}
	if err := envConfig.ValidateWithDefaults(); err != nil {
		log.Fatalf("Invalid .env config: %v", err)
	}

	bot, err := tgbot.New(envConfig.TelegramToken, time.Duration(envConfig.EditWaitSeconds*int(time.Second)))
	if err != nil {
		log.Fatalf("Couldn't start Telegram bot: %v", err)
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		bot.Stop()
		os.Exit(0)
	}()

	log.Printf("Started Telegram bot! Message @%s to start.", bot.Username)

	for update := range bot.GetUpdatesChan() {
		if update.Message == nil {
			continue
		}

		var (
			updateChatID    = update.Message.Chat.ID
			updateMessageID = update.Message.MessageID
			updateUserID    = int64(update.Message.From.ID) // Исправлено! Теперь int64
		)

		if len(envConfig.TelegramID) != 0 && !envConfig.HasTelegramID(updateUserID) {
			log.Printf("User %d is not allowed to use this bot", updateUserID)
			bot.Send(updateChatID, updateMessageID, "You are not authorized to use this bot.")
			continue
		}

		var text string
		switch update.Message.Command() {
		case "help":
			text = "Send a message to start talking with ChatGPT. Use /reload to reset conversation history."
		case "start":
			text = "Welcome! Use /pay_lava to initiate payment."
		case "reload":
			chatGPT.ResetConversation(updateChatID)
			text = "Started a new conversation."
		case "pay_lava":
			paymentLink := generateLavaPaymentLink(updateUserID)
			text = fmt.Sprintf("Оплатите по ссылке: %s", paymentLink)
		case "check_payment":
			if checkPaymentStatus(updateUserID) {
				text = "Оплата подтверждена! Ваш код доступа: 322455"
			} else {
				text = "Оплата не найдена. Попробуйте ещё раз позже."
			}
		default:
			text = "Unknown command. Send /help to see available commands."
		}

		if _, err := bot.Send(updateChatID, updateMessageID, text); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}

func generateLavaPaymentLink(userID int64) string { // Тут тоже исправлено на int64
	return fmt.Sprintf("https://api.lava.ru/pay?amount=100&key=%s&user_id=%d", LAVA_API_KEY, userID)
}

func checkPaymentStatus(userID int64) bool { // Тут тоже исправлено на int64
	response, err := http.Get(fmt.Sprintf("https://api.lava.ru/status?user_id=%d&key=%s", userID, LAVA_API_KEY))
	if err != nil {
		return false
	}
	defer response.Body.Close()

	body, _ := ioutil.ReadAll(response.Body)
	return strings.Contains(string(body), "paid")
}
