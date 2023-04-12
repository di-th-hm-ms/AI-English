package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	// "strconv"
	// "time"

	"github.com/di-th-hm-ms/AI-English/lib"
	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"
)

var bot *lib.LineBot

var userId = ""

type Job struct {
	ID     string
	UserID string
	Data   string
}

func main() {
	// set up log retention
	maxAgeDays := 1
	go func() {
		ticker := time.Tick(24 * time.Hour)
		for range ticker {
			if err := lib.DeleteOldLogs(lib.LogDir, maxAgeDays); err != nil {
				log.Println("Error deleting old log files:", err)
			}
		}
	}()

	// Create a new client for messaging API
	// bot = lib.NewLineBotClient()
	// for debug
	lbot, err := linebot.New(os.Getenv("CHANNEL_SECRET"), "")
	if err != nil {
		// gets this server down temporarily
		log.Fatalf("Failed to create LINE bot client: %v", err)
	}

	bot := &lib.LineBot{
		Client:      lbot,
		AccessToken: "",
	}
	fmt.Println("Success creating a new instance for line bot")

	// Initialize s3 client
	if os.Getenv("PRODUCTION") == "yes" {
		lib.CreateSessionWithRole()
	} else {
		lib.CreateSession()
	}

	router := gin.Default()

	router.POST("/callback", func(c *gin.Context) {

		log.Println("callback is called")
		// validation to limit the scope where http requests are accepted
		lib.WebhookHandler(c, bot.Client)

		// return
		events, err := bot.Client.ParseRequest(c.Request)
		defer c.Request.Body.Close()

		if err != nil {
			if err == linebot.ErrInvalidSignature {
				c.Writer.WriteHeader(400)
				lib.LogWebhookInfo(c, 400)
			} else {
				c.Writer.WriteHeader(500)
				lib.LogWebhookInfo(c, 500)
			}
			return
		}
		for _, event := range events {
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					log.Println("-----------------")
					log.Println(len(events))
					log.Println(message.Text)

					// clean up the input
					sanitizedText, isSanitized := lib.IsEnglishSentence(lib.RemoveExtraSpace(message.Text))
					if !isSanitized {
						if _, err = bot.Client.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("Don't use invalid characters. You can only use english, '.', ',' or space")).Do(); err != nil {
							if _, err = bot.Client.ReplyMessage(event.ReplyToken,
								linebot.NewTextMessage("Sorry, we're under maintenance. Try it later.")).Do(); err != nil {
							}
						}
					}
					log.Println("sanitized")
					log.Println(sanitizedText)

					key := fmt.Sprintf("users/%s/messages/%s", event.Source.UserID, sanitizedText)

					// key := fmt.Sprintf("messages/%s/%s", event.Source.UserID, message.ID)
					// check if this message is duplicated and repeatedly sent.
					if _, isRepeated := lib.GetMessage(key); isRepeated {
						log.Println("repeated")
						botKey := fmt.Sprintf("bots/users/%s/messages/%s", event.Source.UserID, sanitizedText)
						// get past data from s3 to reply
						if content, exists := lib.GetMessage(botKey); exists {

							// send the image retrived from s3 to run efficiently
							imageKey := fmt.Sprintf("bots/users/%s/images/%s", event.Source.UserID, sanitizedText)
							urls := make([]string, 0)
							urls = append(urls, lib.GeneratePresignedUrl(imageKey))
							// reply images
							err = replyPresignedUrl(bot, event, urls)
							// send the past data retrived from s3 to save the cost of gpt
							// To avoid sending messages without user's input due to repeated same requests from LINE server
							if err == nil {
								if _, err = bot.Client.PushMessage(event.Source.UserID,
									linebot.NewTextMessage(string(content))).Do(); err != nil {
									log.Println("Failed to reply a past post: ", err.Error())
								}
								recordUserMessage(event, key, message.ID)
							}

						} else if _, err = bot.Client.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("Sorry, we're in trouble. Wait a moment to recover.")).Do(); err != nil {
							log.Println("Internal server error while replying against repeated request.")
						}
						continue
					}

					// save userId and messageId into s3
					recordUserMessage(event, key, message.ID)
					// Check that this user already used three times
					prefix := fmt.Sprintf("users/%s/messages/", userId)
					todaysCnt, err := lib.CheckThreeTimes(prefix)
					if err != nil {
						log.Println("failed to check for this reason: ", err.Error())

						// emergency reply
						if _, err = bot.Client.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("Sorry, we're under maintenance. Try it later.")).Do(); err != nil {
							log.Println("Failed to reply about a maximum limit warning: ", err.Error())
						}
						continue
					}
					log.Println(todaysCnt)
					if todaysCnt >= 15 {
						if _, err = bot.Client.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("Free users are limited to up to 3 requests per day! Please pay to extend the limit or wait until tomorrow or b")).Do(); err != nil {
							log.Println("Failed to reply about a maximum limit warning: ", err.Error())
						}
						continue
					}
					log.Println("cnt: " + strconv.Itoa(todaysCnt))

					// ask openai of something
					res, err := lib.GetOpenaiChatResponse(sanitizedText)
					// var resStr string // Actual respoonse
					if err != nil || len(res.Choices) == 0 {
						log.Println("an error during gpt api: " + err.Error())
						// delete message data the user sent from s3 because it has no reply
						lib.DeleteObject(key)
						// Reply an error message to the user
						if _, err := bot.Client.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("Sorry, we're in trouble. Wait for recovery.")).Do(); err != nil {
							log.Println("an error while replying an error message from LINE bot" + err.Error())
						}

					} else if len(res.Choices) > 0 {

						// Todo - send multiply for paid users
						// get presigned urls for LINE server to get an access to s3
						// presignedUrls := lib.ScrapeImages(sanitizedText, 1, event.Source.UserID)
						// if len(presignedUrls) > 0 {
						// 	if _, err := bot.Client.ReplyMessage(event.ReplyToken,
						// 		linebot.NewImageMessage(presignedUrls[0], presignedUrls[0])).Do(); err != nil {
						// 		log.Println("an error while replying images from LINE bot" + err.Error())
						// 	}
						// }
						presignedUrls := lib.ScrapeImages(sanitizedText, 1, event.Source.UserID)
						replyPresignedUrl(bot, event, presignedUrls)

						// Send crash course
						if _, err = bot.Client.PushMessage(event.Source.UserID,
							linebot.NewTextMessage(res.Choices[0].Messages.Content)).Do(); err != nil {
							log.Println("an error while replying texts from LINE bot" + err.Error())
						}
						// save this replying data into s3
						key = fmt.Sprintf("bots/users/%s/messages/%s", event.Source.UserID, sanitizedText)
						lib.SaveMessageIdsIntoS3(key, res.Choices[0].Messages.Content)
					}

					// }

				case *linebot.ImageMessage:
					// for both types of users
					key := fmt.Sprintf("users/%s/imageMessages/%s", event.Source.UserID, message.ID)
					data := fmt.Sprintf(`{userId: %s, messageId: %s}`, event.Source.UserID, message.ID)
					lib.SaveMessageIdsIntoS3(key, data)

					// for paid users

					// err := lib.UploadImageFromMessageToS3(bot, key, message)
					// if err != nil {
					// 	bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Error occured while uploading: "+err.Error())).Do()
					// }
					// bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Image uploaded successfully")).Do()

				}
			}
		}

		c.Status(http.StatusOK)
		lib.LogWebhookInfo(c, http.StatusOK)
	})

	go lib.RefreshTokenPeriodically(bot, time.Hour)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

func replyPresignedUrl(bot *lib.LineBot, event *linebot.Event, presignedUrls []string) error {
	if len(presignedUrls) > 0 {
		log.Println(event.ReplyToken)
		log.Println(presignedUrls)
		log.Println(bot.Client)
		if presignedUrls[0] == "" {
			presignedUrls[0] = "https://noimage.com"
			if _, err := bot.Client.ReplyMessage(event.ReplyToken,
				linebot.NewImageMessage(presignedUrls[0], presignedUrls[0])).Do(); err != nil {
				return errors.New("an error while replying images from LINE bot" + err.Error())
			}
		} else if _, err := bot.Client.ReplyMessage(event.ReplyToken,
			linebot.NewImageMessage(presignedUrls[0], presignedUrls[0])).Do(); err != nil {
			return errors.New("an error while replying images from LINE bot" + err.Error())
		}
	}
	return nil
}

func recordUserMessage(event *linebot.Event, key string, id string) {
	// save userId and messageId into s3
	data := fmt.Sprintf(`{userId: %s, messageId: %s}`, event.Source.UserID, id)
	lib.SaveMessageIdsIntoS3(key, data)
}
