package lib

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/line/line-bot-sdk-go/linebot"
)

var bucket = "linenglish"

// s3 region
var awsRegion = "ap-northeast-1"

var s3Client *s3.S3

// take or begin to have power for role
func assumeRole(roleArn, externalId string) (*sts.Credentials, error) {
	sess := session.Must(session.NewSession())
	stsClient := sts.New(sess)

	params := &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleArn),
		RoleSessionName: aws.String("linenglish"),
		ExternalId:      aws.String(externalId),
		DurationSeconds: aws.Int64(3600),
	}

	res, err := stsClient.AssumeRole(params)
	if err != nil {
		return nil, err
	}

	return res.Credentials, nil
}
func CreateSessionWithRole() {
	roleName := os.Getenv("IAM_ROLE_NAME")
	roleId := os.Getenv("IAM_ROLE_ID")
	log.Println(roleName)
	log.Println(roleId)
	roleArn := fmt.Sprintf("arn:aws:iam::%s:role/%s", roleId, roleName)
	externalID := "1234"

	// Initially take a power for role.
	creds, err := assumeRole(roleArn, externalID)
	if err != nil {
		log.Printf("Failed to assume role: %v\n", err)
		return
	}

	// create a temporary session
	sess := session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			*creds.AccessKeyId,
			*creds.SecretAccessKey,
			*creds.SessionToken,
		),
	}))
	s3Client = s3.New(sess)

	// start a goroutine to refresh credentials periodically
	go refreshCredentialsPeriodically(roleArn, externalID, sess)
}

func refreshCredentialsPeriodically(roleArn, externalId string, sess *session.Session) {
	for {
		creds, err := assumeRole(roleArn, externalId)
		if err != nil {
			log.Printf("Failed to assume role: %v\n", err)
		} else {
			exp := aws.TimeValue(creds.Expiration)
			log.Printf("Assumed role successfully. Credentials expire at %v\n", exp)

			// update session with new credentials
			sess.Config.Credentials = stscreds.NewCredentials(sess, roleArn,
				func(arp *stscreds.AssumeRoleProvider) {
					arp.ExternalID = aws.String(externalId)
				})
			// leave s3client as it is

			// Sleep for a duration before refreshing credentials again
			sleepDuration := time.Until(exp.Add(-10 * time.Minute))
			time.Sleep(sleepDuration)
		}
	}
}

func CreateSession() {

	creds := credentials.NewStaticCredentials(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), "")

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
func GetMessage(key string) ([]byte, bool) {
	res, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		log.Println("failed to read object content: " + err.Error())
		return nil, false
	}
	// Read object content
	content, err := io.ReadAll(res.Body)
	if err != nil {
		log.Println("failed to parse data from s3: " + err.Error())
		return nil, false
	}
	return content, true

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

func DeleteObject(key string) error {
	// Delete the object
	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errors.New("failed to delete object: " + err.Error())
	}

	// Confirm if the object was deleted
	err = s3Client.WaitUntilObjectNotExists(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errors.New("failed to confirm object deletion: " + err.Error())
	}

	return nil
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

func GeneratePresignedUrl(key string) string {
	// Generate a presigned URL for the image
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	url, err := req.Presign(5 * time.Minute)
	if err != nil {
		log.Println("Failed to generate presigned URL", err)
		return ""
	}

	log.Println("presigned url: " + url)
	return url
}
