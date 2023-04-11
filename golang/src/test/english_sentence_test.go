package test

import (
	"testing"

	"github.com/di-th-hm-ms/AI-English/lib"
)

func TestIsEnglishSentence(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"This is an English sentence.", true},
		{"क्या आपको यह जानकारी उपयोगी लगी?", false},
		{"こんにちは、元気ですか?", false},
		{"Hello, こんにちは!", false},
		{"12345, 67890", false},
		{"Hello, World! 12345", false},
		{"Hello,            world.", true},
	}

	for _, testCase := range testCases {
		_, result := lib.IsEnglishSentence(testCase.input)
		if result != testCase.expected {
			t.Errorf("isEnglishSentence('%s') = %v; want %v", testCase.input, result, testCase.expected)
		}
	}
}
