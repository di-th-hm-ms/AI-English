package lib

import (
	"fmt"
	"log"
	"strings"

	// "github.com/aws/aws-sdk-go/aws"
	// "github.com/aws/aws-sdk-go/service/s3"
	"github.com/gocolly/colly/v2"
)

func ScrapeImages(keyword string, desiredNumImages int, userId string) []string {
	// Create a collector
	c := colly.NewCollector()

	// Counter to keep track of the number of images
	imgCounter := 0

	// an array for img url
	var presignedUrls []string

	// Find and extract image URLs
	// var comprehesiveErr error
	// var presigned string = ""
	c.OnHTML("img", func(e *colly.HTMLElement) {

		if imgCounter >= desiredNumImages {
			return
		}

		imgURL := e.Attr("src")
		if strings.HasPrefix(imgURL, "http") {
			log.Println("Found image URL:", imgURL)
			// the name of picture is used as the name of this object
			key := fmt.Sprintf("bots/users/%s/images/%s", userId, keyword)
			err := UploadImage(imgURL, key)
			if err != nil {
				log.Println("an error while uploading: " + err.Error())
				// comprehesiveErr = err
			}

			url := GeneratePresignedUrl(key)
			presignedUrls = append(presignedUrls, url)

			imgCounter++

		}
	})

	// if comprehesiveErr != nil {
	// 	log.Println("There's something wrong : " + comprehesiveErr.Error())
	// }

	// Set up error handling
	c.OnError(func(r *colly.Response, err error) {
		log.Println("Request URL:", r.Request.URL, "failed with response:", r, "\nError:", err)
	})

	// Send an HTTP request to Google Images
	escapedKeyword := strings.ReplaceAll(keyword, " ", "+")
	escapedKeyword += "+meaning"
	err := c.Visit("https://www.google.com/search?q=" + escapedKeyword + "&tbm=isch")
	if err != nil {
		log.Println("Error visiting URL:", err)
	}

	return presignedUrls
}
