package lib

import (
	"strings"
)

func removeExtraSpace(keyword string) string {
	keyword = strings.Trim(keyword, " ")

	// Replace all occurrences of more than two consecutive spaces with a single space
	for strings.Contains(keyword, "  ") {
		keyword = strings.ReplaceAll(keyword, "  ", " ")
	}

	return keyword

}

// gpt check

// stripe
