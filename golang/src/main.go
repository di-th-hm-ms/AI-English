package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	// "strconv"
	// "time"

	"github.com/di-th-hm-ms/AI-English/lib"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/linebot"
)

var bot *lib.LineBot

var requests chan *lib.LineRequest

type Job struct {
	ID     string
	UserID string
	Data   string
}

func main() {

	// Load .env file
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Println("Error loading .env file")
	}
	err = godotenv.Load("./.env")
	if err != nil {
		log.Println("Error loading .env file for prod")
	}

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

	isProd := os.Getenv("PRODUCTION") != ""
	// Create a new client for messaging API
	// for deploy
	if isProd {
		log.Println("PRODUCTION")
		lib.NewLineBotClient()
	} else {
		lib.InitializeLinebotDebug()
	}
	bot = lib.GetBot()

	log.Println("Success creating a new instance for line bot")
	log.Println(bot)

	// Initialize s3 client
	if isProd {
		lib.CreateSessionWithRole()
	} else {
		lib.CreateSession()
	}

	// Buffered channel for request queue
	requests = make(chan *lib.LineRequest, 10)

	// Create a worker pool
	workerCnt := 5
	var wg sync.WaitGroup
	wg.Add(workerCnt)

	for i := 0; i < workerCnt; i++ {
		go lib.Worker(requests, &wg)
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
				// add requests to the buffer (channel)
				requests <- &lib.LineRequest{
					UserId:  event.Source.UserID,
					Payload: event,
				}
			}
		}

		c.Status(http.StatusOK)
		lib.LogWebhookInfo(c, http.StatusOK)
	})

	go lib.RefreshTokenPeriodically(time.Hour)

	port := os.Getenv("PORT")
	if port == "" {
		if isProd {
			port = "443" // Default port for HTTPS
		} else {
			port = "8080" // Default port for dev
		}
	}

	if isProd {
		router.RunTLS(":"+port, "", "")
	} else {
		// Dev
		router.Run(":" + port)
	}
}
