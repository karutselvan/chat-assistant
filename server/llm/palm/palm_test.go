package palm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "cloud.google.com/go/ai/generativelanguage/apiv1/generativelanguagepb"

	"github.com/karutselvan/chat-assistant/server/env"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func createResponse(statusCode int, resp proto.Message) (func(w http.ResponseWriter, req *http.Request), error) {
	body, err := protojson.Marshal(resp)
	if err != nil {
		return nil, err
	}

	doer := func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("Body:", string(body))
		w.Write(body)
	}
	return doer, nil
}

func TestGenerateText(t *testing.T) {
	message := "Hello World"
	var index int32 = 0
	doer, err := createResponse(200, &pb.GenerateContentResponse{
		Candidates: []*pb.Candidate{
			{
				Index: &index,
				Content: &pb.Content{
					Parts: []*pb.Part{
						{Data: &pb.Part_Text{Text: message}},
					},
					Role: "",
				},
			},
		},
	})
	if err != nil {
		t.Error(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(http.HandlerFunc(doer)))
	defer ts.Close()

	e := &env.Environment{
		PalmApiKey:       "",
		DatabaseHostname: "",
		DatabaseUserName: "",
		DatabasePassword: "",
		Platform:         env.GOTEST,
	}
	ctx := context.Background()
	client, err := NewPalmLLMClient(ctx, e, option.WithoutAuthentication(), option.WithEndpoint(ts.URL))
	if err != nil {
		t.Error(err)
	}

	resp, err := client.GenerateText(ctx, "test")
	if err != nil {
		t.Error(err)
	}

	if string(resp) != message {
		t.Errorf("Expected '%s' got '%s'", message, string(resp))
	}
}
