package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var lastResultMessageID int   // переменная для хранения ID последнего сообщения с результатом
var running bool = false      // флаг для проверки, работает ли горутина
var messageIDs []int          // для хранения ID всех сообщений
var stopChannel chan struct{} // канал для остановки горутины
var modeMessageID int         // Для хранения ID сообщения с режимом (Авто/Ручной)
var dataMessageID int         // Для хранения ID сообщения "Получение данных"
var mu sync.Mutex             // мьютекс для синхронизации доступа к горутине и сообщениями

func main() {
	initMQTTClient()
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Panic("TELEGRAM_BOT_TOKEN environment variable not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	messageChannel := make(chan string)

	//Меню
	for update := range updates {
		if update.Message != nil {
			if update.Message.Text == "/start" {
				// Создание inline-клавиатуры
				inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Автоматический режим", "callback_data_1"),
						tgbotapi.NewInlineKeyboardButtonData("Ручной режим", "callback_data_2"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("Получить данные", "callback_data_3"),
					),
				)

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите действие:")
				msg.ReplyMarkup = inlineKeyboard
				bot.Send(msg)
			}
		}

		// Обработка нажатия на кнопки
		if update.CallbackQuery != nil {
			callback := update.CallbackQuery

			if modeMessageID != 0 {
				deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, modeMessageID)
				bot.Send(deleteMsg)
				modeMessageID = 0 // сбрасываем ID после удаления
			}

			// Удаляем сообщение "Получение данных", если оно было отправлено
			if dataMessageID != 0 {
				deleteMsg := tgbotapi.NewDeleteMessage(callback.Message.Chat.ID, dataMessageID)
				bot.Send(deleteMsg)
				dataMessageID = 0 // Сбрасываем ID после удаления
			}

			var responseText string

			switch callback.Data {
			case "callback_data_1":
				responseText = "Включён автоматический режим"
				stopGoroutine(bot, callback.Message.Chat.ID)
				sendData(true)
				sendData(true)

			case "callback_data_2":
				responseText = "Включён ручной режим"
				stopGoroutine(bot, callback.Message.Chat.ID)
				running = false
				sendData(false)
				sendData(false)
			case "callback_data_3":
				// responseText = "Получение данных"
				go subscribeToTopic("vadlap/topic", messageChannel, true)
				if dataMessageID == 0 {
					responseText = "Получение данных"
				}
				startGoroutine(bot, callback.Message.Chat.ID, messageChannel)
			}

			// Отправляем новое сообщение с результатом и сохраняем его ID
			if responseText != "" {
				newMsg := tgbotapi.NewMessage(callback.Message.Chat.ID, responseText)
				sentMsg, _ := bot.Send(newMsg)
				lastResultMessageID = sentMsg.MessageID // Сохраняем ID отправленного сообщения

				// Если выбраны режимы "Автоматический" или "Ручной", сохраняем ID сообщения
				if callback.Data == "callback_data_1" || callback.Data == "callback_data_2" || callback.Data == "callback_data_3" {
					modeMessageID = sentMsg.MessageID // Сохраняем ID сообщения с текстом режима
				}
			}

			// Ответ на нажатие кнопки (отправка CallbackQuery через bot.Request)
			callbackResponse := tgbotapi.NewCallback(callback.ID, responseText)
			if _, err := bot.Request(callbackResponse); err != nil {
				log.Println("Error sending callback response:", err)
			}
		}
	}
}

// Функция для запуска горутины
func startGoroutine(bot *tgbotapi.BotAPI, chatID int64, messageChannel chan string) {
	if running {
		log.Println("Goroutine already run")
		return
	}
	stopChannel = make(chan struct{}) // Создаем новый канал для остановки горутины
	running = true                    // Устанавливаем флаг работы

	go func() {
		for {
			select {
			case <-stopChannel: // Остановка горутины
				return
			case message := <-messageChannel:
				fmt.Print("\n" + message)

				mu.Lock() // Блокировка для синхронизации доступа
				infMsg := tgbotapi.NewMessage(chatID, message)
				sentMsg, _ := bot.Send(infMsg)
				messageIDs = append(messageIDs, sentMsg.MessageID) // Сохраняем ID сообщения
				mu.Unlock()
			}
		}
	}()
}

// Функция для остановки горутины и удаления сообщений
func stopGoroutine(bot *tgbotapi.BotAPI, chatID int64) {
	if !running {
		log.Println("Горутина не запущена, нечего останавливать.")
		return
	}
	close(stopChannel) // Отправляем сигнал на остановку горутины
	running = false    // Сбрасываем флаг работы

	// Удаляем все отправленные сообщения
	mu.Lock()
	for _, msgID := range messageIDs {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, msgID)
		bot.Send(deleteMsg)
	}
	messageIDs = []int{} // Очищаем список сообщений
	mu.Unlock()
}
