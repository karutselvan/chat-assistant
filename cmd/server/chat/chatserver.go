package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/davecgh/go-spew/spew"
	"github.com/karutselvan/chat-assistant/server"
	"github.com/karutselvan/chat-assistant/server/db"
	"github.com/karutselvan/chat-assistant/server/env"
	"github.com/karutselvan/chat-assistant/server/llm"
	"github.com/karutselvan/chat-assistant/server/llm/kernel"
	"github.com/karutselvan/chat-assistant/server/llm/palm"

	pb "google.golang.org/api/chat/v1"
)

const defaultChatAppProject = "1007744422436"
const chatIssuer = "chat@system.gserviceaccount.com"
const jwtURL = "https://www.googleapis.com/service_accounts/v1/jwk/"

type ChatHandler struct {
	verifier  *oidc.IDTokenVerifier
	llm       llm.LlmClient
	db        db.EmbeddingsDB
	projectID string
	kernel    kernel.Kernel
}

func NewChatHandler(ctx context.Context, environment *env.Environment) (*ChatHandler, error) {
	// TODO: query the metadata server if we're on cloud run.
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = defaultChatAppProject
	}

	slog.Info("Using Cloud", "projectID", projectID)

	config := &oidc.Config{
		SkipClientIDCheck: true,
		ClientID:          projectID,
	}
	ks := oidc.NewRemoteKeySet(ctx, jwtURL+chatIssuer)
	verifier := oidc.NewVerifier(chatIssuer, ks, config)

	kernel, err := kernel.NewHandRolledKernel(ctx, environment)
	if err != nil {
		return nil, err
	}

	llm, err := palm.NewPalmLLMClient(ctx, environment)
	if err != nil {
		return nil, err
	}

	edb, err := db.NewPostgresDatabase(environment)
	if err != nil {
		return nil, err
	}

	return &ChatHandler{
		verifier:  verifier,
		llm:       llm,
		db:        edb,
		projectID: projectID,
		kernel:    kernel,
	}, err
}

// Validate the Chat Token

func (handler *ChatHandler) validateChatToken(context context.Context, tokenString string) error {

	payload, err := handler.verifier.Verify(context, tokenString)
	if err != nil {
		slog.Error("handler.verifier.Verify failed: ", "error", err)
		return err
	}
	var claims struct {
		Aud string `json:"aud"`
		Iss string `json:"iss"`
	}
	if err := payload.Claims(&claims); err != nil {
		slog.Error("payload.Claims failed: ", "error", err)
		return err
	}

	fmt.Printf("\n\nAud: %s; Iss: %s\n\n", claims.Aud, claims.Iss)
	if claims.Aud != handler.projectID {
		return fmt.Errorf("audience was not the correct chat project, got '%s' expected '%s'", claims.Aud, handler.projectID)
	}
	if claims.Iss != chatIssuer {
		return fmt.Errorf("issuer was not '%s'; got: '%s'", chatIssuer, claims.Iss)
	}
	return nil
}

// CHAT

func CreateResponseCard(cardId, sessionId, text, uri string) (*pb.Message, error) {
	button := pb.GoogleAppsCardV1Button{
		Text: "Learn More",
		OnClick: &pb.GoogleAppsCardV1OnClick{
			OpenLink: &pb.GoogleAppsCardV1OpenLink{
				Url: uri + "/chat/action/12344",
			},
		},
	}
	// <a href="https://www.flaticon.com/free-icons/thumbs-down" title="thumbs down icons">Thumbs down icons created by Freepik - Flaticon</a>

	thumbsUpButton := pb.GoogleAppsCardV1Button{
		Icon: &pb.GoogleAppsCardV1Icon{
			AltText:   "Thumbs Up",
			IconUrl:   "https://storage.cloud.google.com/robsite-assistant-public/thumb-up.png",
			ImageType: "SQUARE",
		},
		//Text: "Thumbs Up",
		OnClick: &pb.GoogleAppsCardV1OnClick{
			OpenLink: &pb.GoogleAppsCardV1OpenLink{
				Url: fmt.Sprintf("%s/feedback/up/%s", uri, sessionId),
			},
		},
	}
	thumbsDownButton := pb.GoogleAppsCardV1Button{
		//Text: "Thumbs Down",
		Icon: &pb.GoogleAppsCardV1Icon{
			AltText:   "Thumbs Down",
			IconUrl:   "https://storage.cloud.google.com/robsite-assistant-public/thumb-down.png",
			ImageType: "SQUARE",
		},
		OnClick: &pb.GoogleAppsCardV1OnClick{
			OpenLink: &pb.GoogleAppsCardV1OpenLink{
				Url: fmt.Sprintf("%s/feedback/down/%s", uri, sessionId),
			},
		},
	}
	card := &pb.CardWithId{
		Card: &pb.GoogleAppsCardV1Card{
			Header: &pb.GoogleAppsCardV1CardHeader{
				ImageType: "CIRCLE",
				ImageUrl:  "https://developers.google.com/chat/images/chat-product-icon.png",
				Subtitle:  "Created with the Robsite Assistant",
				Title:     "Robsite Assistant Reply",
			},
			Sections: []*pb.GoogleAppsCardV1Section{{
				Widgets: []*pb.GoogleAppsCardV1Widget{{
					TextParagraph: &pb.GoogleAppsCardV1TextParagraph{
						Text: text,
					},
				}, {
					ButtonList: &pb.GoogleAppsCardV1ButtonList{
						Buttons: []*pb.GoogleAppsCardV1Button{
							&button, &thumbsUpButton, &thumbsDownButton},
					},
				}},
			}},
		},
		CardId: cardId,
	}

	resp := &pb.Message{
		CardsV2: []*pb.CardWithId{card},
	}
	return resp, nil
}

func (handler *ChatHandler) DebugCard(w http.ResponseWriter, r *http.Request) {
	uri := server.GetPublicEndpoint(r)
	slog.Info("URI: " + uri)
	if reqText, err := httputil.DumpRequest(r, true); err == nil {
		slog.Info("[DebugCard] Request: " + string(reqText))
	}
	r.URL.Query()

	sessionId := r.URL.Query().Get("id")
	text := r.URL.Query().Get("text")
	if text == "" {
		text = "This is a test"
	}
	resp, err := CreateResponseCard("TestCard", sessionId, text, uri)
	if err != nil {
		resp = &pb.Message{
			Text: "Error creating Card",
		}
	}
	server.EncodeAndLogResponse(resp, w)
}

type BasicChat struct {
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

func (handler *ChatHandler) HandleChatBasic(w http.ResponseWriter, r *http.Request) {
	uri := server.GetPublicEndpoint(r)
	slog.Info("HandleChatBasic URI: " + uri)

	req := &BasicChat{}
	json.NewDecoder(r.Body).Decode(&req)
	slog.Info(fmt.Sprint("Decoded Message: ", spew.Sdump(req)))

	sessionId := "0"
	text, err := handler.kernel.Chat(r.Context(), req.Name, sessionId, req.Text)
	if err != nil {
		slog.Error("Error: ", "error", err)
		server.EncodeAndLogResponse(&pb.Message{
			Text: "Error: " + err.Error(),
		}, w)
		return
	}

	server.EncodeAndLogResponse(&pb.Message{
		Text: text,
	}, w)

}

func (handler *ChatHandler) HandleChatApp(w http.ResponseWriter, r *http.Request) {
	uri := server.GetPublicEndpoint(r)
	slog.Info("URI: " + uri)
	if reqText, err := httputil.DumpRequest(r, true); err == nil {
		slog.Info(string(reqText))
	}
	ctx := r.Context()

	if authHeader, ok := r.Header["Authorization"]; ok {
		token := strings.Split(authHeader[0], " ")[1]
		if err := handler.validateChatToken(ctx, token); err != nil {
			fmt.Printf("Error validating Token: %v\n", err)
			fmt.Fprintf(w, "Error validating Token: %v\n", err)
			return
		}
	} else {
		// TODO return error to client here.
		fmt.Println("No Auth Header!")
		return
	}

	req := pb.DeprecatedEvent{}
	json.NewDecoder(r.Body).Decode(&req)
	fmt.Printf("Decoded Message: %#v\n", req)

	if req.Message == nil || req.Message.Sender == nil {
		fmt.Println("No Message or Sender")
		return
	}

	name := req.Message.Sender.DisplayName

	// todo - make a real session id
	sessionId := "0"

	text, err := handler.kernel.Chat(r.Context(), name, sessionId, req.Message.ArgumentText)
	if err != nil {
		slog.Error("Error in handleChat: ", "error", err)
		server.EncodeAndLogResponse(&pb.Message{
			Text: "Error creating Card",
		}, w)
		return
	}

	resp, err := CreateResponseCard("ChatResponseCard", sessionId, text, uri)
	if err != nil {
		slog.Error("Error creating card (CreateResponseCard): ", "error", err)
		server.EncodeAndLogResponse(&pb.Message{
			Text: "Error creating Card",
		}, w)
		return
	}

	server.EncodeAndLogResponse(resp, w)
}

func (handler *ChatHandler) Close() {
	if handler.llm != nil {
		handler.llm.Close()
	}
}
