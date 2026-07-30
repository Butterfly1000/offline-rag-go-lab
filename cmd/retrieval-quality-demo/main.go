package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: retrieval-quality-demo <dataset|index|sparse|hybrid|rerank|policy> [flags]")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	var err error
	switch os.Args[1] {
	case "dataset":
		err = runDataset(ctx, os.Args[2:], os.Stdout)
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}
