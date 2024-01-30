package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/karutselvan/chat-assistant/server"
	"github.com/karutselvan/chat-assistant/server/db"
	"github.com/karutselvan/chat-assistant/server/env"
	"github.com/karutselvan/chat-assistant/server/llm"
	"github.com/karutselvan/chat-assistant/server/llm/kernel"
	"github.com/karutselvan/chat-assistant/server/llm/palm"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

const defaultChatAppProject = "392420558361"

// const chatIssuer = "chat@system.gserviceaccount.com"
// const jwtURL = "https://www.googleapis.com/service_accounts/v1/jwk/"

type SlackHandler struct {
	llm           llm.LlmClient
	db            db.EmbeddingsDB
	api           *slack.Client
	clientId      string
	clientSecret  string
	signingSecret string
	projectID     string
	kernel        kernel.Kernel
}

func NewSlackHandler(ctx context.Context, environment *env.Environment, router *mux.Router) (*SlackHandler, error) {
	// TODO: query the metadata server if we're on cloud run.
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = defaultChatAppProject
	}

	slog.Info("NewSlackHandler: Using Cloud", "projectID", projectID)

	llm, err := palm.NewPalmLLMClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	edb, err := db.NewPostgresDatabase(environment)
	if err != nil {
		return nil, err
	}

	api := slack.New(environment.SlackBotOAuthToken, slack.OptionDebug(true))

	kernel, err := kernel.NewHandRolledKernel(ctx, environment)
	if err != nil {
		return nil, err
	}

	handler := &SlackHandler{
		llm:           llm,
		db:            edb,
		projectID:     projectID,
		api:           api,
		clientId:      environment.SlackClientID,
		clientSecret:  environment.SlackClientSecret,
		signingSecret: environment.SlackSigningSecret,
		kernel:        kernel,
	}

	// TODO - remove GET from these
	router.HandleFunc("/commands/help", handler.slashHelp).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc("/action-endpoint", handler.actionEndpoint).Methods(http.MethodPost, http.MethodGet)

	// This is jsut here for testing
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("Slack handler is active.\n")) }).Methods(http.MethodGet)

	return handler, err
}

func (handler *SlackHandler) slashHelp(w http.ResponseWriter, r *http.Request) {
	// TODO publish help
	uri := server.GetPublicEndpoint(r)
	slog.Info("SlackHandler:slashHelp URI: " + uri)

	ctx := r.Context()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.ErrorContext(ctx, "SlackHandler:slashHelp", "error", err)
		return
	}
	var prettyJSON bytes.Buffer
	if err = json.Indent(&prettyJSON, body, "", "\t"); err != nil {
		slog.ErrorContext(ctx, "JSON parse error", "error", err)
		return
	}
	slog.InfoContext(ctx, "SlackHandler:slashHelp", "request", prettyJSON.String())
}

func (handler *SlackHandler) handleAppMentionEvent(ctx context.Context, ev slackevents.AppMentionEvent) {
	text := ev.Text
	if _, rest, found := strings.Cut(text, "> "); found {
		text = rest
	}

	response, err := handler.kernel.Chat(ctx, ev.User, ev.EventTimeStamp, text)
	if err != nil {
		msg := fmt.Sprintf("Error: %v", err.Error())
		handler.api.PostMessage(ev.Channel, slack.MsgOptionText(msg, true))
		return
	}
	channel, ts, err := handler.api.PostMessage(ev.Channel, slack.MsgOptionText(response, false))
	if err != nil {
		slog.ErrorContext(ctx, "error posting message to channel", "err", err)
		return
	}
	slog.InfoContext(ctx, "posted message", "channel", channel, "timestamp", ts, "response", response)
}

func (handler *SlackHandler) handleURLVerification(ctx context.Context, w http.ResponseWriter, body []byte) {
	var r *slackevents.ChallengeResponse
	if err := json.Unmarshal(body, &r); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.ErrorContext(ctx, "SlackHandler:actionEndpoint error Unmarshal event", "error", "StatusInternalServerError")
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(r.Challenge))
	slog.InfoContext(ctx, "Responded with Challenge")
}

func (handler *SlackHandler) actionEndpoint(w http.ResponseWriter, r *http.Request) {
	// TODO publish help

	// ctx := r.Context()
	// See if this fixes the cancelled errors
	ctx := context.Background()
	uri := server.GetPublicEndpoint(r)
	slog.InfoContext(ctx, "SlackHandler:actionEndpoint called", "URI", uri)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.ErrorContext(ctx, "SlackHandler:actionEndpoint", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	sv, err := slack.NewSecretsVerifier(r.Header, handler.signingSecret)
	if err != nil {
		slog.ErrorContext(ctx, "SlackHandler:actionEndpoint", "error", "StatusBadRequest")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := sv.Write(body); err != nil {
		slog.ErrorContext(ctx, "SlackHandler:actionEndpoint", "error", "StatusInternalServerError")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// TODO - this is failing, fix soon
	// if err := sv.Ensure(); err != nil {
	// 	w.WriteHeader(http.StatusUnauthorized)
	// 	return
	// }

	slog.InfoContext(ctx, "SlackHandler:actionEndpoint", "request", string(body))

	// TODO - verify token here?
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.ErrorContext(ctx, "SlackHandler:actionEndpoint error parsing event", "error", "StatusInternalServerError")
		return
	}

	slog.InfoContext(ctx, "Event type: "+eventsAPIEvent.Type)
	switch eventsAPIEvent.Type {
	case slackevents.URLVerification:
		handler.handleURLVerification(ctx, w, body)
	case slackevents.CallbackEvent:
		switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			go handler.handleAppMentionEvent(ctx, *ev)
		default:
			slog.InfoContext(ctx, "unhandled event type", "ev", ev)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func (handler *SlackHandler) Close() {
	if handler.llm != nil {
		handler.llm.Close()
	}
}
