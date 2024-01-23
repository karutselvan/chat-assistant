package main

import (
	"context"
	"fmt"
	"os"

	"github.com/karutselvan/chat-assistant/server/env"
	"github.com/karutselvan/chat-assistant/server/llm/palm"
)

func main() {
	args := os.Args[1:]

	env, err := env.NewEnvironmentForPlatform(env.COMMANDLINE)
	if err != nil {
		fmt.Printf("ERROR: %v\n\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	client, err := palm.NewPalmLLMClient(ctx, env)
	if err != nil {
		fmt.Printf("ERROR: %v\n\n", err)
		os.Exit(2)
	}
	defer client.Close()

	for _, arg := range args {
		response, err := client.GenerateText(ctx, arg)
		if err != nil {
			fmt.Printf("ERROR: %v\n\n", err)
			continue
		}
		fmt.Println(response)
	}

}
