package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m1guelpf/chatgpt-telegram/src/adapters"
	"github.com/m1guelpf/chatgpt-telegram/src/chatgpt"
	"github.com/m1guelpf/chatgpt-telegram/src/config"
	"github.com/m1guelpf/chatgpt-telegram/src/entities"
	"github.com/m1guelpf/chatgpt-telegram/src/session"
	"github.com/m1guelpf/chatgpt-telegram/src/tgbot"
)

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

	fk := adapters.NewFreeKassaProvider(envConfig.FreeKassaMerchantID, envConfig.FreeKassaSecret1, envConfig.FreeKassaSecret2, envConfig.FreeKassaAPIKey)

	bot, err := tgbot.New(envConfig.TelegramToken, time.Duration(envConfig.EditWaitSeconds)*time.Second)
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

		updateChatID := update.Message.Chat.ID
		updateMessageID := update.Message.MessageID
		updateUserID := update.Message.From.ID

		if len(envConfig.TelegramID) != 0 && !envConfig.HasTelegramID(int64(updateUserID)) {
			log.Printf("User %d is not allowed to use this bot", updateUserID)
			bot.Send(updateChatID, updateMessageID, "You are not authorized to use this bot.")
			continue
		}

		var text string
		switch update.Message.Command() {
		case "help":
			text = "Send a message to start talking with ChatGPT. Use /reload to reset conversation history."
		case "start":
			text = "Welcome! Use /pay to initiate payment."
		case "reload":
			chatGPT.ResetConversation(updateChatID)
			text = "Started a new conversation."
		case "pay":
			ctx := context.Background()
			resp, err := fk.CreatePayment(ctx, entities.PaymentRequest{
				OrderID: int(updateUserID),
				Amount:  100.0,
				Email:   fmt.Sprintf("user%d@example.com", updateUserID),
			})
			if err != nil {
				text = "Ошибка при создании платежа."
				break
			}
			text = fmt.Sprintf("Оплатите по ссылке: %s", resp.PaymentURL)
		case "check_payment":
			ctx := context.Background()
			resp, err := fk.VerifyPayment(ctx, entities.PaymentVerificationRequest{
				OrderID: fmt.Sprintf("%d", updateUserID),
			})
			if err != nil {
				text = "Ошибка при проверке платежа."
				break
			}
			if resp.IsPaid {
				text = fmt.Sprintf("✅ Платёж подтверждён. Сумма: %s", resp.Amount)
			} else {
				text = "⏳ Платёж ещё не оплачен."
			}
		default:
			text = "Unknown command. Send /help to see available commands."
		}

		if _, err := bot.Send(updateChatID, updateMessageID, text); err != nil {
			log.Printf("Error sending message: %v", err)
		}
	}
}
