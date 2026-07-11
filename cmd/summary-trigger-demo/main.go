package main

import (
	"flag"
	"fmt"
	"log"

	"offline-rag-go-lab/internal/sessionsummary"
)

func main() {
	messages := flag.Int("messages", 8, "number of messages after the summary watermark")
	tokens := flag.Int("tokens", 1000, "tokens after the summary watermark")
	evicted := flag.Int("evicted", 2, "unsummarized messages outside the recent window")
	minMessages := flag.Int("min-messages", 8, "message threshold for summary generation")
	minTokens := flag.Int("min-tokens", 2048, "token threshold for summary generation")
	flag.Parse()

	policy, err := sessionsummary.NewTriggerPolicy(*minMessages, *minTokens)
	if err != nil {
		log.Fatal(err)
	}
	decision, err := policy.Decide(sessionsummary.TriggerInput{
		UnsummarizedMessages: *messages,
		UnsummarizedTokens:   *tokens,
		EvictedMessages:      *evicted,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Unsummarized messages: %d\n", *messages)
	fmt.Printf("Unsummarized tokens: %d\n", *tokens)
	fmt.Printf("Evicted messages: %d\n", *evicted)
	fmt.Printf("Minimum messages: %d\n", *minMessages)
	fmt.Printf("Minimum tokens: %d\n", *minTokens)
	fmt.Printf("Should summarize: %t\n", decision.ShouldSummarize)
	fmt.Printf("Reason: %s\n", decision.Reason)
}
