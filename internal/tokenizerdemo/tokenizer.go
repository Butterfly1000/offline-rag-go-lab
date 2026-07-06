package tokenizerdemo

import (
	"fmt"
	"strings"

	"github.com/sugarme/tokenizer/pretrained"
)

type Message struct {
	Role    string
	Content string
}

type Counter struct {
	path string
}

func NewCounter(tokenizerPath string) Counter {
	return Counter{path: tokenizerPath}
}

func (c Counter) CountText(text string) (count int, tokens []string, ids []int, err error) {
	tk, err := pretrained.FromFile(c.path)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("load tokenizer from %q: %w", c.path, err)
	}

	encoding, err := tk.EncodeSingle(text, false)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("encode text: %w", err)
	}

	return encoding.Len(), encoding.Tokens, encoding.Ids, nil
}

func (c Counter) CountMessages(messages []Message) (perMessage []int, total int, transcriptCount int, err error) {
	var transcript strings.Builder
	perMessage = make([]int, 0, len(messages))

	for _, msg := range messages {
		count, _, _, countErr := c.CountText(msg.Content)
		if countErr != nil {
			return nil, 0, 0, countErr
		}
		perMessage = append(perMessage, count)
		total += count

		transcript.WriteString(msg.Role)
		transcript.WriteString(": ")
		transcript.WriteString(msg.Content)
		transcript.WriteString("\n")
	}

	transcriptCount, _, _, err = c.CountText(transcript.String())
	if err != nil {
		return nil, 0, 0, err
	}

	return perMessage, total, transcriptCount, nil
}
