package lib

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
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

const OpenaiURL = "https://api.openai.com/v1/chat/completions"

// Including user's inpput and gpt's response
var Conversation []Message

// Get the crash course to user's input.
func GetOpenaiChatResponse(input string) (*OpenaiResponse, error) {
	apiKey := os.Getenv("GPT_KEY")
	Conversation = append(Conversation, Message{
		Role: "user",
		// Content: `Teach me the meaning of the next word and show me
		//  couple of short conversations including as many as phrasal verbs,
		//  slangs and the next word in the conversations. "` + input + `"`,
		Content: `Let me know the meaning about ` + input + ` concisely without any extra explanations`,
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
	req, err := http.NewRequest("POST", OpenaiURL, bytes.NewBuffer(reqJson))
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
