package lib

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/linebot"
)

const (
	LogDir  = "logs"
	LogFile = "webhook_logs.txt"
)

func LogWebhookInfo(c *gin.Context, statusCode int) {

	// Ensure the log directory exists
	if err := os.MkdirAll(LogDir, 0755); err != nil {
		log.Println("Error creating log directory:", err)
		return
	}
	// Open the log file for appending, create it if it doesn't exist
	filePath := filepath.Join(LogDir, LogFile)
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening log file:", err)
		return
	}
	defer file.Close()

	// Format the log line and write it to the log file
	logLine := fmt.Sprintf("%s\t%s\t%s\t%s\t%d\n",
		c.ClientIP(), time.Now().Format(time.RFC1123),
		c.Request.Method, c.Request.URL.Path, statusCode)

	if _, err := file.WriteString(logLine); err != nil {
		log.Println("Error writing to log file:", err)
	}

}

func DeleteOldLogs(logDir string, maxAgeDays int) error {
	files, err := ioutil.ReadDir(logDir)
	if err != nil {
		return err
	}

	now := time.Now()
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour

	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), LogFile) {
			fileAge := now.Sub(file.ModTime())
			if fileAge > maxAge {
				filePath := filepath.Join(logDir, file.Name())
				if err := os.Remove(filePath); err != nil {
					return err
				}
				log.Printf("Deleted old log file: %s\n", filePath)
			}
		}
	}

	return nil
}

func WebhookHandler(c *gin.Context, bot *linebot.Client) {
	// http && post req
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		log.Println("couldn't read the request body")
		return
	}

}
