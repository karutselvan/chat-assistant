package embedcmd

import (
	"context"
	"fmt"

	"github.com/rcleveng/assistant/server/db"
	"github.com/rcleveng/assistant/server/env"
	"github.com/rcleveng/assistant/server/llm/palm"
	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the n closest matches",
	RunE: func(cmd *cobra.Command, args []string) error {
		env, err := env.NewEnvironmentForPlatform(env.COMMANDLINE)
		if err != nil {
			return err
		}

		for _, arg := range args {
			if err := query(env, arg); err != nil {
				return err
			}
		}

		return nil
	},
}

var (
	count int
)

func init() {
	RootCmd.AddCommand(queryCmd)
	queryCmd.LocalFlags().IntVar(&count, "count", 1, "number of closest matches to find")
}

func query(env *env.Environment, text string) error {
	ctx := context.Background()
	llm, err := palm.NewPalmLLMClient(ctx, env)
	if err != nil {
		return err
	}
	defer llm.Close()

	embeddings, err := llm.EmbedText(ctx, text)
	if err != nil {
		return err
	}

	var edb db.EmbeddingsDB

	edb, err = db.NewPostgresDatabase(env)
	if err != nil {
		return err
	}
	defer edb.Close()

	matches, err := edb.Find(embeddings, count)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		fmt.Println("no matches")
		return nil
	}

	for _, m := range matches {
		fmt.Println("Match:", m)
	}

	return nil
}
