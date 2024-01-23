package chat

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v3"
	"github.com/karutselvan/chat-assistant/server/db"
	pb "google.golang.org/api/chat/v1"
)

// from jwks_test.go
type signingKey struct {
	keyID string // optional
	priv  interface{}
	pub   interface{}
	alg   jose.SignatureAlgorithm
}

func newRSAKey(t testing.TB) *signingKey {
	priv, err := rsa.GenerateKey(rand.Reader, 1028)
	if err != nil {
		t.Fatal(err)
	}
	return &signingKey{"", priv, priv.Public(), jose.RS256}
}

// func (s *signingKey) jwk() jose.JSONWebKey {
// 	return jose.JSONWebKey{Key: s.pub, Use: "sig", Algorithm: string(s.alg), KeyID: s.keyID}
// }

// sign creates a JWS using the private key from the provided payload.
func (s *signingKey) sign(t testing.TB, payload []byte) string {
	privKey := &jose.JSONWebKey{Key: s.priv, Algorithm: string(s.alg), KeyID: s.keyID}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: s.alg, Key: privKey}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	data, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestLlmClient - LLM Client for testing
type TestLlmClient struct {
	Opened      bool
	LastPrompt  string
	PromptCount int
}

func (c *TestLlmClient) GenerateText(ctx context.Context, prompt string) (string, error) {
	c.LastPrompt = prompt
	c.PromptCount++
	return prompt, nil
}

func (c *TestLlmClient) Close() error {
	c.Opened = false
	return nil
}

func (c *TestLlmClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func (c *TestLlmClient) BatchEmbedText(ctx context.Context, text []string) ([][]float32, error) {
	return nil, nil
}

func NewChatHandlerForTest(keySet *oidc.StaticKeySet, llm *TestLlmClient) *ChatHandler {
	config := &oidc.Config{
		SkipClientIDCheck: true,
		ClientID:          defaultChatAppProject,
	}
	verifier := oidc.NewVerifier(chatIssuer, keySet, config)
	edb := db.NoopEmbeddingsDB{}
	return &ChatHandler{
		verifier: verifier,
		llm:      llm,
		db:       edb,
		// This needs to match the clientid above
		projectID: defaultChatAppProject,
	}
}

func createIdToken(t *testing.T, key *signingKey) string {
	exp := time.Now().Add(time.Hour)
	payload := []byte(fmt.Sprintf(`{ "iss": "%s", "aud": "%s", "exp": %d}`, chatIssuer, defaultChatAppProject, exp.Unix()))

	idToken := key.sign(t, payload)
	return idToken
}

func TestSmoke(t *testing.T) {
	key := newRSAKey(t)
	keySet := oidc.StaticKeySet{PublicKeys: []crypto.PublicKey{key.pub}}

	r := &pb.DeprecatedEvent{
		Common: &pb.CommonEventObject{
			HostApp:    "CHAT",
			UserLocale: "en",
		},
		ConfigCompleteRedirectUrl: "https://chat.google.com/api/bot_config_complete?token=REDACTED",
		EventTime:                 "2023-11-14T05:58:46.651279Z",
		IsDialogEvent:             false,
		Message: &pb.Message{
			ArgumentText:  "Hello World",
			FormattedText: "@MyBot Hello World",
			Name:          "spaces/SPACE_NAME/messages/MESSAGE_ID",
			Sender:        &pb.User{},
			Text:          "@MyBot Hello World",
		},
		Type: "MESSAGE",
	}
	body, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		t.Fatal(err)
	}

	llm := TestLlmClient{
		Opened:     true,
		LastPrompt: "",
	}

	request := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(string(body)))

	idToken := createIdToken(t, key)
	request.Header.Set("Authorization", "Bearer "+idToken)

	response := httptest.NewRecorder()

	handler := NewChatHandlerForTest(&keySet, &llm)
	handler.HandleChatApp(response, request)

	if !strings.Contains(llm.LastPrompt, "USERQUESTION: Hello World") {
		t.Errorf("Expected prompt to be Hello World, got %s", llm.LastPrompt)
	}

	if !llm.Opened {
		t.Error("Expected LLM to be opened still.")
	}

}
