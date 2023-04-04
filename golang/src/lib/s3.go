package lib

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/line/line-bot-sdk-go/linebot"
)

var bucket = "linenglish"

// s3 region
var awsRegion = "ap-northeast-1"

var s3Client *s3.S3

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
	log.Println("S3's connection works")

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

func UploadImageFromMessageToS3(bot *linebot.Client, key string, message *linebot.ImageMessage) error {
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

func UploadImage(url string, key string) error {
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Error downloading image:", err)
		return err
	}
	defer resp.Body.Close()

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading image data:", err)
		return err
	}

	// filename := strings.Split(url, "/")
	// key := keyword + "/" + filename[len(filename)-1]

	log.Println("upload image")
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(imageData),
	})
	if err != nil {
		log.Println("an error uploading image picked up to S3")
	}

	log.Println("after uploading")
	return nil
}

func DeleteAll() {

	// Call the ListObjectsV2 API to get a list of all objects in the bucket
	res, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})

	// Create a slice to store object keys
	var objKeys []*s3.ObjectIdentifier

	for _, obj := range res.Contents {
		objKeys = append(objKeys, &s3.ObjectIdentifier{
			Key: obj.Key,
		})
	}

	if len(objKeys) == 0 {
		log.Println("no objects found in bucket")
	}
	_, err = s3Client.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3.Delete{
			Objects: objKeys,
		},
	})
	if err != nil {
		log.Println("failed to delete obs")
	}

}

// check if this user already used this system three times
func CheckThreeTimes(prefix string) (int, error) {

	today := time.Now().Truncate(24 * time.Hour)
	// endTime := startTime.Add(24 * time.Hour)

	res, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return 0, err
	}
	// todaysObjects := make([]*s3.Object, 0)
	todaysCnt := 0
	for _, object := range res.Contents {
		if object.LastModified.After(today) {
			// todaysObjects = append(todaysObjects, object)
			todaysCnt++
			// log.Println(aws.StringValue(object.Key))
		}
	}

	return todaysCnt, nil
}
