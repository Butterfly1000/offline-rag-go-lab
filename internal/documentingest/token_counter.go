package documentingest

import "offline-rag-go-lab/internal/tokenizerdemo"

type TokenCounter interface {
	Count(text string) (int, error)
}

type qwenTokenCounter struct {
	counter *tokenizerdemo.Counter
}

func NewQwenTokenCounter(path string) (TokenCounter, error) {
	counter, err := tokenizerdemo.LoadCounter(path)
	if err != nil {
		return nil, err
	}
	return &qwenTokenCounter{counter: counter}, nil
}

func (c *qwenTokenCounter) Count(text string) (int, error) {
	count, _, _, err := c.counter.CountText(text)
	return count, err
}
