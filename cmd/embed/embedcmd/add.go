package embedcmd

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"github.com/karutselvan/chat-assistant/server/db"
	"github.com/karutselvan/chat-assistant/server/env"
	"github.com/karutselvan/chat-assistant/server/llm"
	"github.com/karutselvan/chat-assistant/server/llm/palm"
	"github.com/spf13/cobra"
	"github.com/tmc/langchaingo/textsplitter"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new text and embedding",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, err := env.NewEnvironmentForPlatform(env.COMMANDLINE)
		if err != nil {
			return err
		}
		if err := add(env, args); err != nil {
			return err
		}

		return nil
	},
}

var author int64

func init() {
	RootCmd.AddCommand(addCmd)
	addCmd.LocalFlags().Int64Var(&author, "author", 0, "sets the author's email address ")
}

func add(env *env.Environment, text []string) error {
	ctx := context.Background()
	llm, err := palm.NewPalmLLMClient(ctx, env)
	if err != nil {
		return err
	}
	defer llm.Close()

	splitter := textsplitter.NewRecursiveCharacter()
	splitter.ChunkOverlap = 20
	splitter.ChunkSize = 1000

	var edb db.EmbeddingsDB

	if UseDatabase {
		fmt.Println("Using postgresql database")
		edb, err = db.NewPostgresDatabase(env)
		if err != nil {
			return err
		}
	} else {
		fmt.Println("Skipping database")
		edb = db.NoopEmbeddingsDB{}
	}
	defer edb.Close()

	if err = embedAndAdd(ctx, splitter, llm, edb, text); err != nil {
		return err
	}

	return nil
}

type Splitter interface {
	SplitText(text string) ([]string, error)
}

func embedAndAdd(ctx context.Context, splitter Splitter, lm llm.LlmClient, db db.EmbeddingsDB, texts []string) error {
	splits := make([]string, 0, len(texts))
	for _, text := range texts {
		cursplits, err := splitter.SplitText(text)
		if err != nil {
			return err
		}
		splits = append(splits, cursplits...)
	}

	resp, err := lm.BatchEmbedText(ctx, splits)
	if err != nil {
		fmt.Printf("ERROR: %v\n\n", err)
	}

	glog.V(2).Info(spew.Sdump(resp))

	for i, e := range resp {
		author := int64(0)
		if i < len(splits) {

			glog.V(1).Infof("Embedding: [%d] [%#v] '%s']\n", i, e, splits[i])
			if _, err := db.Add(author, splits[i], e); err != nil {
				return err
			}
		} else {
			fmt.Printf("past end of splits with i=%d\n", i)
			fmt.Printf("Past Split Embedding: [%d] [%#v]]\n", i, e)
		}
	}
	return nil
}
