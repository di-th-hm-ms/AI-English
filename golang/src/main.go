package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// "strconv"
	// "time"

	"github.com/di-th-hm-ms/AI-English/lib"
	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"
)

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
	bot, err := linebot.New(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_ACCESS_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success creating a new instance for line bot")

	// Initialize s3 client
	lib.CreateSession()

	router := gin.Default()

	router.POST("/callback", func(c *gin.Context) {

		log.Println("callback is called")
		// validation to limit the scope where http requests are accepted
		lib.WebhookHandler(c, bot)

		// return
		events, err := bot.ParseRequest(c.Request)
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

					// key := fmt.Sprintf("messages/%s/%s", event.Source.UserID, message.ID)
					key := fmt.Sprintf("users/%s/messages/%s", event.Source.UserID, message.Text)
					// check if this message is duplicated and repeatedly sent.
					if lib.IsRepeated(key) {
						log.Println("repeated")
						continue
					}

					// save userId and messageId into s3
					data := fmt.Sprintf(`{userId: %s, messageId: %s}`, event.Source.UserID, message.ID)
					lib.SaveMessageIdsIntoS3(key, data)

					// ask openai of something
					res, err := lib.GetOpenaiChatResponse(message.Text)
					// var resStr string // Actual respoonse
					if err != nil || len(res.Choices) == 0 {
						log.Println("an error during gpt api: " + err.Error())
					} else if len(res.Choices) > 0 {

						// Todo - send multiply for paid users
						// get presigned urls
						presignedUrls := lib.ScrapeImages(message.Text, 1, event.Source.UserID)
						if len(presignedUrls) > 0 {
							if _, err := bot.ReplyMessage(event.ReplyToken,
								linebot.NewImageMessage(presignedUrls[0], presignedUrls[0])).Do(); err != nil {
								log.Println("an error while replying images from LINE bot" + err.Error())
							}
						}

						// Send crash course
						if _, err = bot.PushMessage(event.Source.UserID,
							linebot.NewTextMessage(res.Choices[0].Messages.Content)).Do(); err != nil {
							log.Println("an error while replying texts from LINE bot" + err.Error())
						}
						// save this replying data into s3
						key = fmt.Sprintf("bots/users/%s/messages/%s", event.Source.UserID, message.Text)
						lib.SaveMessageIdsIntoS3(key, res.Choices[0].Messages.Content)
					}

					// }

				case *linebot.ImageMessage:
					// key := fmt.Sprintf("%s/images/%s.jpg", event.Source.UserID, strconv.FormatInt(time.Now().UnixNano(), 10))
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
