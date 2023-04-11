package lib

import (
	"regexp"
	"strings"
)

func RemoveExtraSpace(keyword string) string {
	keyword = strings.Trim(keyword, " ")

	// Replace all occurrences of more than two consecutive spaces with a single space
	for strings.Contains(keyword, "  ") {
		keyword = strings.ReplaceAll(keyword, "  ", " ")
	}

	return keyword

}

// check if input is an English sentence or not.
func IsEnglishSentence(input string) (string, bool) {
	// Define a regular expression to match English letters and spaces
	regExp := regexp.MustCompile(`[^a-zA-Z\s,.?]`)
	// Clean the input string by removing non-English letters and converting it to lowercase
	cleanedInput := strings.ToLower(regExp.ReplaceAllLiteralString(input, ""))

	// Calculate the frequency of English characters
	englishFrequency := float64(len(cleanedInput)) / float64(len(input))

	// Return true if the frequency of English characters is above a certain threshold (e.g., 70%)
	if englishFrequency >= 1 {
		return cleanedInput, true
	}
	return input, false
}

// gpt check

// stripe
