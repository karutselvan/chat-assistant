package kernel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/karutselvan/chat-assistant/server/db"
	"github.com/karutselvan/chat-assistant/server/env"
	"github.com/karutselvan/chat-assistant/server/llm"
	"github.com/karutselvan/chat-assistant/server/llm/palm"
)

type HandRolledKernel struct {
	llm llm.LlmClient
	db  db.EmbeddingsDB
}

func NewHandRolledKernel(ctx context.Context, environment *env.Environment) (*HandRolledKernel, error) {
	llm, err := palm.NewPalmLLMClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	edb, err := db.NewPostgresDatabase(environment)
	if err != nil {
		return nil, err
	}

	return &HandRolledKernel{
		llm: llm,
		db:  edb,
	}, nil
}

func (k *HandRolledKernel) RunChain(ctx context.Context, cmd, rest, name string) (string, error) {
	switch cmd {
	case "ANSWER":
		return rest, nil
	case "NEEDMORE":
		// TODO - ask this as a follow up, then add the next answer to the context and
		// retry the original question
		return rest, nil
	case "CALENDAR":
		return fmt.Sprintf("I would use the calendar to look up '%s'", rest), nil
	case "REMEMBER":
		emb, err := k.llm.EmbedText(ctx, rest)
		if err != nil {
			return rest, fmt.Errorf("error trying to remember (emb): '%s'", rest)
		}
		_, err = k.db.Add(0, rest, emb)
		if err != nil {
			return rest, fmt.Errorf("error trying to remember (db): '%s'", rest)
		}
		return fmt.Sprintf("I will remember that '%s'", rest), nil
	default:
		return cmd + " " + rest, nil
	}

}

func (k *HandRolledKernel) Chat(ctx context.Context, name, sessionId, text string) (string, error) {

	slog.Info(fmt.Sprint("Kernel Chat: ", spew.Sdump(ctx)))
	emb, err := k.llm.EmbedText(ctx, text)
	slog.Info(fmt.Sprint("Embed Obj: ", spew.Sdump(emb)))
	if err != nil {
		return "", err
	}

	context, err := k.db.Find(emb, 2)
	slog.Info(fmt.Sprint("Context Obj: ", spew.Sdump(context)))
	if err != nil {
		context = []string{}
	}

	prompt, err := llm.ChatPrompt(text, context)
	slog.Info(fmt.Sprint("Prompt Obj: ", spew.Sdump(prompt)))
	if err != nil {
		fmt.Println("error generating chat prompt", err.Error())
		prompt = text
	}
	responseText, err := k.llm.GenerateText(ctx, prompt)
	slog.Info(fmt.Sprint("Generated text Obj: ", spew.Sdump(responseText)))
	if err != nil {
		return "", err
	}

	// TODO - process response for remembering, looking up calendar and starting chain, etc
	cmd, rest, found := strings.Cut(responseText, " ")
	if found && cmd[len(cmd)-1:] == ":" {
		// we have a command
		cmd = cmd[:len(cmd)-1]
		responseText, err = k.RunChain(ctx, cmd, rest, name)
		if err != nil {
			fmt.Println("running chain failed ", err.Error())
			return "", err
		}
	}
	return responseText, nil
}

func (k *HandRolledKernel) Close() error {
	if k.llm != nil {
		return k.llm.Close()
	}
	return nil
}
