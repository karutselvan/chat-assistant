package chat

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"

	"github.com/gorilla/mux"
	"github.com/karutselvan/chat-assistant/server"

	pb "google.golang.org/api/chat/v1"
)

type FeedbackHandler struct {
}

func (handler *FeedbackHandler) HandleFeedback(w http.ResponseWriter, r *http.Request) {
	uri := server.GetPublicEndpoint(r)
	slog.Info("URI: " + uri)
	if reqText, err := httputil.DumpRequest(r, true); err == nil {
		slog.Info(string(reqText))
	}

	vars := mux.Vars(r)
	feedbackType := vars["type"]    // 'up' or 'down'
	feedbackSessionId := vars["id"] // Id to apply feedback

	resp := &pb.Message{Text: fmt.Sprintf("Received '%s' feedback on id: '%s'", feedbackType, feedbackSessionId)}
	server.EncodeAndLogResponse(resp, w)
}

func (handler *FeedbackHandler) Close() {
}
