package url_service

import (
	"errors"
	"io"
	"regexp"
	"strings"
)

var ErrPreviewUnsupported = errors.New("preview not supported by this previewer")

type previewResult struct {
	Url         string
	SiteName    string
	Type        string
	Description string
	Title       string
	Image       *previewImage
}

type previewImage struct {
	ContentType         string
	Data                io.ReadCloser
	Filename            string
	ContentLength       int64
	ContentLengthHeader string
}

func summarize(text string, maxWords int, maxLength int) (string) {
	// Normalize the whitespace to be something useful (crush it to one giant line)
	surroundingWhitespace := regexp.MustCompile(`^[\s\p{Zs}]+|[\s\p{Zs}]+$`)
	interiorWhitespace := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	newlines := regexp.MustCompile(`[\r\n]`)
	text = surroundingWhitespace.ReplaceAllString(text, "")
	text = interiorWhitespace.ReplaceAllString(text, " ")
	text = newlines.ReplaceAllString(text, " ")

	words := strings.Split(text, " ")
	result := text
	if len(words) >= maxWords {
		result = strings.Join(words[:maxWords], " ")
	}

	if len(result) > maxLength {
		// First try trimming off the last word
		words = strings.Split(result, " ")
		newResult := words[0]
		for _, word := range words {
			if len(newResult+" "+word) > maxLength {
				break
			}
			newResult = newResult + " " + word
		}
		result = newResult
	}

	if len(result) > maxLength {
		// It's still too long, just trim the thing and add an ellipsis
		result = result[:maxLength] + "..."
	}

	return result
}
