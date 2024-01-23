// Sample run-helloworld is a minimal Cloud Run service.
package docs

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/karutselvan/chat-assistant/apps"
	"github.com/karutselvan/chat-assistant/cards"
	"github.com/karutselvan/chat-assistant/server"
)

func mainCard(w http.ResponseWriter, r *http.Request, event *apps.WorkspaceAppEvent) *cards.Card {
	uri := fmt.Sprintf("%s/authorizeFile", getURI(r))

	sampleButton := &cards.Button{
		Text: "Sample Button",
		OnClick: &cards.OnClick{
			Action: &cards.Action{
				Function: uri,
			},
		},
	}
	card := &cards.Card{
		// CardActions: []*cards.CardAction{},
		// Header:      &cards.CardHeader{},
		Name: "Sample Card",
		Sections: []*cards.Section{{
			Header: "",
			Widgets: []*cards.WidgetMarkup{{
				ButtonList:          &cards.ButtonList{Buttons: []*cards.Button{sampleButton}},
				HorizontalAlignment: "CENTER",
				// Image:      &cards.Image{},
				// TextParagraph: &cards.TextParagraph{
				// 	Text: "Hello World",
				// },
			}},
		}},
	}

	if event == nil || event.CommonEventObject == nil {
		return card
	}
	switch event.CommonEventObject.HostApp {
	case "DOCS":
		if event.Docs.Id == nil {
			// Need to auth
			button := &cards.Button{
				Text: "Authorize File",
				OnClick: &cards.OnClick{
					Action: &cards.Action{
						Function: uri,
					},
				},
			}
			card.FixedFooter = &cards.CardFixedFooter{
				PrimaryButton: button,
			}
		}
	}

	return card
}

func AuthFileHandler(w http.ResponseWriter, r *http.Request) {
	response := cards.SubmitFormResponse{
		RenderAction: &cards.RenderActions{
			HostAppAction: &cards.HostAppAction{
				EditorAction: &cards.EditorClientAction{
					RequestFileScopeForActiveDocument: cards.RequestFileScopeForActiveDocument{},
				},
			},
		},
	}

	server.EncodeAndLogResponse(&response, w)
}

func getURI(r *http.Request) string {
	proto := "http"
	if protos, ok := r.Header["X-Forwarded-Proto"]; ok {
		proto = protos[0]
	}
	return fmt.Sprintf("%s://%s", proto, r.Host)
}

func DocsHandler(w http.ResponseWriter, r *http.Request) {

	uri := getURI(r)
	slog.Info("URI: " + uri)
	if reqText, err := httputil.DumpRequest(r, true); err == nil {
		slog.Info(string(reqText))
	}

	event := apps.WorkspaceAppEvent{}
	json.NewDecoder(r.Body).Decode(&event)

	mc := mainCard(w, r, &event)
	action := cards.RenderActions{
		Action: &cards.RenderActionsAction{
			Navigations: &[]cards.Navigation{
				{PushCard: mc}}},
	}

	server.EncodeAndLogResponse(&action, w)
}
