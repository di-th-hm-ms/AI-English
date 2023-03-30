package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var bucket = "linenglish"

// s3 region
var awsRegion = "ap-northeast-1"

var s3Client *s3.S3

func main() {
	bot, err := linebot.New(
		os.Getenv("CHANNEL_SECRET"),
		os.Getenv("CHANNEL_ACCESS_TOKEN"),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Success creating a new instance for line bot")

	// Initialize s3 client
	CreateSession()

	// Delete Bucket
	// DeleteBucket()

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
				case *linebot.ImageMessage:
					key := fmt.Sprintf("images/%s/%s.jpg", event.Source.UserID, strconv.FormatInt(time.Now().UnixNano(), 10))

					err := uploadImageToS3(bot, key, message, bucket)
					if err != nil {
						bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Error occured while uploading: "+err.Error())).Do()
					}
					bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("Image uploaded successfully")).Do()

				}
			}
		}
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}

func CreateSession() {

	creds := credentials.NewStaticCredentials(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_ACCESS_KEY"), "")

	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String("ap-northeast-1")},
	)
	if err != nil {
		log.Fatal("Fail to create a new session")
	}
	s3Client = s3.New(sess)
	log.Println("complete!!")

}

func uploadImageToS3(bot *linebot.Client, key string, message *linebot.ImageMessage, bucketName string) error {
	// Get image data from LINE Messaging API
	response, err := bot.GetMessageContent(message.ID).WithContext(context.Background()).Do()
	if err != nil {
		return err
	}
	defer response.Content.Close()
	imageBytes, err := io.ReadAll(response.Content)
	if err != nil {
		return err
	}

	// Upload image to S3
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(bucketName),
		Key:           aws.String(key),
		Body:          bytes.NewReader(imageBytes),
		ContentType:   aws.String(http.DetectContentType(imageBytes)),
		ContentLength: aws.Int64(int64(len(imageBytes))),
		// ACL:           aws.String("public-read"),
	})
	if err != nil {
		return err
	}

	return nil
}

// func uploadImage() {
// params := &s3.PutObjectInput{
// 	Bucket: aws.String(bucket),
// 	Key: aws.String(key),
// 	Body: imageFile,
// }

// if _, err := s3Client.PutObject(params) {
// 	log.Fatal()
// }
// }
