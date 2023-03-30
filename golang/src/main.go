package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"
)

func main() {
	bot, err := linebot.New(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_ACCESS_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success creating a new instance for line bot")

	router := gin.Default()

	router.GET("/hello", func(c *gin.Context) {
		c.String(http.StatusOK, "hello!!")
	})

	router.POST("/callback", func(c *gin.Context) {
		events, err := bot.ParseRequest(c.Request)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				c.Writer.WriteHeader(400)
			} else {
				c.Writer.WriteHeader(500)
			}
			return
		}
		for _, event := range events {
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(message.Text)).Do(); err != nil {
						log.Print(err)
					}
				case *linebot.StickerMessage:
					replyMessage := fmt.Sprintf(
						"sticker id is %s, stickerResourceType is %s", message.StickerID, message.StickerResourceType)
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(replyMessage)).Do(); err != nil {
						log.Print(err)
					}
				}
			}
		}
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
	// http.HandleFunc("/webhook", func(w http.ResponseWriter, req *http.Request) {
	// 	log.Fatal("ping\n")
	// 	events, err := bot.ParseRequest(req)
	// 	if err != nil {
	// 		if err == linebot.ErrInvalidSignature {
	// 			w.WriteHeader(http.StatusBadRequest)
	// 		} else {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 		}
	// 		return
	// 	}

	// 	// manage events
	// 	for _, e := range events {
	// 		log.Fatal(e)
	// 		// the event is to receive a message.
	// 		if e.Type == linebot.EventTypeMessage {
	// 			switch message := e.Message.(type) {
	// 			// text event
	// 			case *linebot.TextMessage:
	// 				replyMessage := linebot.NewTextMessage(message.Text)
	// 				log.Fatal(replyMessage)
	// 				if _, err := bot.ReplyMessage(e.ReplyToken, replyMessage).Do(); err != nil {
	// 					log.Fatal(err)
	// 				}
	// 			}
	// 		}
	// 	}

	// 	w.WriteHeader(200)
	// })

	// port := os.Getenv("PORT")
	// if port == "" {
	// 	port = "8080"
	// }

	// fmt.Println(port)
	// if err := http.ListenAndServe(":"+port, nil); err != nil {
	// 	log.Fatal(err)
	// }
}
