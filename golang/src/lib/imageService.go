package lib

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type SearchResult struct {
	TotalResults int     `json:"total_results"`
	Page         int     `json:"page"`
	PerPage      int     `json:"per_page"`
	Photos       []Photo `json:"photos"`
}

type Photo struct {
	Id              int    `json:"id"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Url             string `json:"url"`
	Photographer    string `json:"photographer"`
	PhotographerUrl string `json:"photographer_url"`
}

var apiKey = os.Getenv("PEXELS_API_KEY")

func GetImagesFromPexels(input string) ([]Photo, error) {

	input = strings.Replace(input, " ", "+", -1)
	q := fmt.Sprintf("https://api.pexels.com/v1/search?query=%s&per_page=1", input)
	req, err := http.NewRequest("GET", q, nil)
	if err != nil {
		return nil, err
	}

	// set auth to header
	req.Header.Set("Authorization", apiKey)

	// exe sending a request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// Parse bytes of JSON to a Map
	var result SearchResult
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	log.Println(result.Photos)

	return result.Photos, nil
}
