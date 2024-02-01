package palm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	generativelanguage "cloud.google.com/go/ai/generativelanguage/apiv1"
	pb "cloud.google.com/go/ai/generativelanguage/apiv1/generativelanguagepb"

	"cloud.google.com/go/secretmanager"

	"github.com/google/generative-ai-go/genai"
	"github.com/karutselvan/chat-assistant/server/env"
	"google.golang.org/api/option"
)

type PalmLLMClient struct {
	// Everything we need context wise from the environment
	environment *env.Environment
	// client to call api
	client    *genai.Client
	genclient *generativelanguage.GenerativeClient
}

func (c *PalmLLMClient) Close() error {
	err := c.client.Close()
	err2 := c.genclient.Close()
	return errors.Join(err, err2)
}

func (c *PalmLLMClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	em := c.client.GenerativeModel("gemini-pro")
	resp, err := em.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}

	if len(resp.Candidates) > 0 {
		s := responseString(resp)
		return s, nil
	}
	return "", fmt.Errorf("no candidate response, just %#v", resp)
}

func responseString(resp *genai.GenerateContentResponse) string {
	var b strings.Builder
	for i, cand := range resp.Candidates {
		if len(resp.Candidates) > 1 {
			fmt.Fprintf(&b, "%d:", i+1)
		}
		b.WriteString(contentString(cand.Content))
	}
	return b.String()
}

func contentString(c *genai.Content) string {
	if c == nil || c.Parts == nil {
		return ""
	}
	var b strings.Builder
	for i, part := range c.Parts {
		if i > 0 {
			fmt.Fprintf(&b, ";")
		}
		fmt.Fprintf(&b, "%v", part)
	}
	return b.String()
}

func (c *PalmLLMClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	em := c.client.EmbeddingModel("models/embedding-001")
	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, err
	}
	if res == nil || res.Embedding == nil || len(res.Embedding.Values) < 10 {
		return nil, fmt.Errorf("empty embeddings returned")
	}

	return res.Embedding.Values, nil
}
func toContent(parts []string) *pb.Content {
	p := make([]*pb.Part, len(parts))
	for i, e := range parts {
		p[i] = &pb.Part{
			Data: &pb.Part_Text{
				Text: e,
			},
		}
	}
	return &pb.Content{Role: "user", Parts: p}
}

func (c *PalmLLMClient) batchEmbedContentWithTitle(ctx context.Context, title string, texts []string) (*pb.BatchEmbedContentsResponse, error) {
	reqs := make([]*pb.EmbedContentRequest, len(texts))
	taskType := pb.TaskType(genai.TaskTypeUnspecified)

	for i, e := range texts {
		slog.InfoContext(ctx, "Adding emb batch", "i", i, "e", e)
		reqs[i] = &pb.EmbedContentRequest{
			Model: "models/embedding-001",
			// TODO - support multiple parts per embedding
			Content:  toContent([]string{e}),
			TaskType: &taskType,
			Title:    &title,
		}
	}

	req := &pb.BatchEmbedContentsRequest{
		Model:    "models/embedding-001",
		Requests: reqs,
	}
	res, err := c.genclient.BatchEmbedContents(ctx, req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *PalmLLMClient) BatchEmbedText(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.batchEmbedContentWithTitle(ctx, "", texts)
	if err != nil {
		return nil, err
	}

	if resp.Embeddings == nil {
		return nil, fmt.Errorf("unable to find embedding json structure")
	}

	embeddings := make([][]float32, 0, len(texts))
	for _, emb := range resp.Embeddings {
		embeddings = append(embeddings, emb.Values)
	}
	return embeddings, nil
}

func NewPalmLLMClient(ctx context.Context, environment *env.Environment, opts ...option.ClientOption) (*PalmLLMClient, error) {

	/*
	client, err := secretmanager.NewClient(context.Background())
  	if err != nil {
    		return nil, err
	}

  	// Access the secret.
  	secretVersion, err := client.AccessSecretVersion(context.Background(), &secretmanager.AccessSecretVersionRequest{
    		Name: "genai-api-key/versions/latest",
  	})
  	if err != nil {
	    	return nil, err
  	}

  	// Get the secret data.
  	secretData = secretVersion.Payload.Data
	slog.Info("Palm API Env Key" + secretData + " -\")
	*/
	allopts := append([]option.ClientOption{option.WithAPIKey("AIzaSyALb82cms3j503kC6YU0z4MXOo4zYNM5KQ")}, opts...)
	client, err := genai.NewClient(ctx, allopts...)
	if err != nil {
		return nil, err
	}

	// TODO - remove this once the go client supports the batchEmbeddings
	genclient, err := generativelanguage.NewGenerativeRESTClient(ctx, allopts...)
	if err != nil {
		return nil, err
	}

	return &PalmLLMClient{
		environment: environment,
		client:      client,
		genclient:   genclient,
	}, nil
}
