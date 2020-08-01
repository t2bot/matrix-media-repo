package util

import (
	"io/ioutil"
	"strings"
	"unicode/utf8"

	"github.com/saintfish/chardet"
	"golang.org/x/net/html/charset"
)

func ToUtf8(text string, possibleContentType string) string {
	if utf8.ValidString(text) {
		return text
	}

	textCharset := ""

	if possibleContentType != "" {
		_, name, ok := charset.DetermineEncoding([]byte(text), possibleContentType)
		if ok {
			textCharset = name
		}
	}

	if textCharset == "" {
		detector := chardet.NewTextDetector()
		cs, err := detector.DetectBest([]byte(text))
		if err != nil {
			return text // best we can do
		}
		textCharset = cs.Charset
	}

	r, err := charset.NewReader(strings.NewReader(text), textCharset)
	if err != nil {
		return text // best we can do
	}

	converted, err := ioutil.ReadAll(r)
	if err != nil {
		return text // best we can do
	}

	return string(converted)
}
