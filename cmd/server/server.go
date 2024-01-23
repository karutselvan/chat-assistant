// Sample run-helloworld is a minimal Cloud Run service.
package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/karutselvan/chat-assistant/cmd/server/chat"
	"github.com/karutselvan/chat-assistant/cmd/server/docs"
	"github.com/karutselvan/chat-assistant/cmd/server/slack"
	"github.com/karutselvan/chat-assistant/server/env"

	"os"
)

// TODO - This may go away if we don't need Environment per-request since the
// handlers have it already.
func addRequestEnvironment(next http.Handler, environment *env.Environment) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := env.NewContext(r.Context(), environment)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

//go:embed static_files/*
var content embed.FS

func staticFiles() (fs.FS, error) {
	s, err := fs.Sub(content, "static_files")
	fs.WalkDir(s, ".", func(path string, d fs.DirEntry, err error) error {
		fmt.Println(path)
		return nil
	})
	return s, err
}

func main() {
	environment, err := env.NewEnvironment()
	if err != nil {
		log.Fatal(err)
	}

	ctx := env.NewContext(context.Background(), environment)
	if environment.Platform == env.CLOUDRUN {
		cloudRunService := os.Getenv("K_SERVICE")
		slog.InfoContext(ctx, "starting server on cloud run: "+cloudRunService)
		env.SetupCloudLogging()
	}

	router := mux.NewRouter()

	chatHandler, err := chat.NewChatHandler(ctx, environment)
	if err != nil {
		log.Fatal(err)
	}

	defer chatHandler.Close()
	feedbackHandler := chat.FeedbackHandler{}

	router.HandleFunc("/docs", docs.DocsHandler).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc("/authorizeFile", docs.AuthFileHandler).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc("/chat", chatHandler.HandleChatApp).Methods(http.MethodPost)
	router.HandleFunc("/chat/basic", chatHandler.HandleChatBasic).Methods(http.MethodPost)
	router.HandleFunc("/debug/card", chatHandler.DebugCard).Methods(http.MethodGet)
	router.HandleFunc("/feedback/{type}/{id}", feedbackHandler.HandleFeedback).Methods(http.MethodPost)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{'o', 'k', '\n'})
	})
	router.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{'o', 'k', '\n'})
	})
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{'o', 'k', '\n'})
	})

	// Add Slack Support
	slackRouter := router.PathPrefix("/slack").Subrouter()

	slackHandler, err := slack.NewSlackHandler(ctx, environment, slackRouter)
	if err != nil {
		log.Fatal(err)
	}
	defer slackHandler.Close()

	// Serve the static files off of root last since gorilla mux cares about the order
	// where stdlib uses prefix length
	sf, err := staticFiles()
	if err == nil {
		fs := http.FileServer(http.FS(sf))
		router.PathPrefix("/").Handler(fs)
	}

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "2302"
	}

	// Start HTTP server.
	slog.Info("listening on port " + port)
	if err := http.ListenAndServe(":"+port, handlers.LoggingHandler(os.Stdout, addRequestEnvironment(router, environment))); err != nil {
		log.Fatal(err)
	}
}
