package tokenizerdemo

import (
	"fmt"
	"strings"

	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

type Message struct {
	Role    string
	Content string
}

type Counter struct {
	tokenizer *tokenizer.Tokenizer
}

func LoadCounter(tokenizerPath string) (*Counter, error) {
	// FromFile reads tokenizer.json and constructs its normalizer, pre-tokenizer,
	// model, post-processor, and decoder. This work is done once at startup.
	tk, err := pretrained.FromFile(tokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer from %q: %w", tokenizerPath, err)
	}

	return newCounter(tk), nil
}

func newCounter(tk *tokenizer.Tokenizer) *Counter {
	return &Counter{tokenizer: tk}
}

func (c *Counter) CountText(text string) (count int, tokens []string, ids []int, err error) {
	// EncodeSingle executes the rules already loaded from tokenizer.json.
	// false means this plain-text count does not add special tokens.
	encoding, err := c.tokenizer.EncodeSingle(text, false)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("encode text: %w", err)
	}

	return encoding.Len(), encoding.Tokens, encoding.Ids, nil
}

func (c *Counter) CountMessages(messages []Message) (perMessage []int, total int, transcriptCount int, err error) {
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
