package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/spf13/cobra"
	pb "google.golang.org/api/chat/v1"
)

var rootCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat is a commandline to the debug chat API",
	Long:  `Command to chat with the debug api`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return chat(args)
	},
}

var (
	debugChatURL string
	verbose      bool
)

func init() {
	rootCmd.PersistentFlags().StringVar(&debugChatURL, "url", "http://localhost:2302/chat/basic", "http endpoint to the debug chat server")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "be verbose")
}

type BasicChat struct {
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

func chat(text []string) error {
	b, err := json.Marshal(BasicChat{
		Name: os.Getenv("LOGNAME"),
		Text: strings.Join(text, "\n"),
	})
	if err != nil {
		return err
	}
	if verbose {
		fmt.Println("Request: ", string(b))
	}

	resp, err := http.Post(debugChatURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("error HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	jsonbytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	chatResp := &pb.Message{}
	json.Unmarshal(jsonbytes, chatResp)

	if verbose {
		fmt.Println("Status: ", resp.StatusCode)
		fmt.Println("Response:", spew.Sdump(chatResp))
	}

	fmt.Println(chatResp.Text)

	return nil
}

func main() {

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}
