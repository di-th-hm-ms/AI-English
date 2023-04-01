package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	// "github.com/di-th-hm-ms/AI-English/lib"
	// "AIEnglish/golang/src/lib"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// openAI
type OpenaiRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type OpenaiResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int      `json:"created"`
	Choices []Choice `json:"choices"`
	Usages  Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Messages     Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LINE bot and S3
type MsgData struct {
	UserID       int `json:"userId"`
	ReqMessageID int `json:"messageId"`
}

const openaiURL = "https://api.openai.com/v1/chat/completions"

var bucket = "linenglish"

// s3 region
var awsRegion = "ap-northeast-1"

var s3Client *s3.S3

// Including user's inpput and gpt's response
var Conversation []Message

var cnt = 0

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

	router := gin.Default()

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
					log.Println("-----------------")
					log.Println(len(events))
					log.Println(message.Text)

					key := fmt.Sprintf("messages/%s/%s", event.Source.UserID, message.ID)
					// check if this message is duplicated and repeatedly sent.
					if IsRepeated(key) {
						log.Println("repeated")
						continue
					}

					// save userId and messageId into s3
					data := fmt.Sprintf(`{userId: %s, messageId: %s}`, event.Source.UserID, message.ID)
					SaveMessageIdsIntoS3(key, data)

					// ask openai of something
					res, err := GetOpenaiChatResponse(message.Text)
					// var resStr string // Actual respoonse
					if err != nil || len(res.Choices) == 0 {
						log.Println("an error during gpt api: " + err.Error())
					} else if len(res.Choices) > 0 {
						if _, err = bot.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage(res.Choices[0].Messages.Content)).Do(); err != nil {
							log.Println("an error while replying from LINE bot" + err.Error())
						}
					}

				case *linebot.ImageMessage:
					key := fmt.Sprintf("images/%s/%s.jpg", event.Source.UserID, strconv.FormatInt(time.Now().UnixNano(), 10))

					err := uploadImageToS3(bot, key, message)
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

func uploadImageToS3(bot *linebot.Client, key string, message *linebot.ImageMessage) error {
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
		Bucket:        aws.String(bucket),
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

/*
*

	These two methods below are for validation to check if the message is repeated
	from LINE API.
*/
func SaveMessageIdsIntoS3(key string, data string) {
	// body := []byte(data)
	body := strings.NewReader(data)
	// Upload the text data to S3
	_, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   aws.ReadSeekCloser(body),
	})
	if err != nil {
		log.Println("Failed to upload text data to S3", err)
	}
}

// To check if the data is already on s3.
func IsRepeated(key string) bool {
	_, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

// Get the crash course to user's input.
func GetOpenaiChatResponse(input string) (*OpenaiResponse, error) {
	apiKey := os.Getenv("GPT_KEY")
	Conversation = append(Conversation, Message{
		Role: "user",
		Content: `Teach me the meaning of the next word and show me
		 couple of short conversations including as many as phrasal verbs, 
		 slangs and the next word in the conversations. "` + input + `"`,
	})
	reqBody := OpenaiRequest{
		Model:    "gpt-3.5-turbo",
		Messages: Conversation,
	}

	// encode Json to string
	reqJson, err := json.Marshal(reqBody)
	if err != nil {
		log.Println("an error while encoding JSON" + err.Error())
		return nil, err
	}

	// create a request to openai
	req, err := http.NewRequest("POST", openaiURL, bytes.NewBuffer(reqJson))
	if err != nil {
		log.Println("an error while creating a request" + err.Error())
		return nil, err
	}

	// set options into a header
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Execute a request to openai
	client := &http.Client{}
	res, err := client.Do(req)
	log.Println("openai req")
	if err != nil {
		log.Println("an error while encoding JSON" + err.Error())
		return nil, err
	}

	// set up io
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("an error while closing io" + err.Error())
		}
	}(res.Body)

	// read the body of response with io reader
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println("an error while reading body of the response with io" + err.Error())
		return nil, err
	}

	// parse(decode) a response which is encoded JSON and store it with 2nd argument
	var openaiRes OpenaiResponse
	err = json.Unmarshal(body, &openaiRes)
	if err != nil {
		log.Println("an error while parsing encoded JSON response" + err.Error())
		return nil, err
	}

	if len(openaiRes.Choices) > 0 {
		Conversation = append(Conversation, Message{
			Role:    "assistant",
			Content: openaiRes.Choices[0].Messages.Content,
		})
	}

	return &openaiRes, nil

}
