package kernel

import (
	"context"
	"io"
)

// LLM Kernel
type ChainRunner interface {
	RunChain(ctx context.Context, cmd, rest, name string) (string, error)
}

type Chatter interface {
	Chat(ctx context.Context, name, sessionId, text string) (string, error)
}

type Kernel interface {
	ChainRunner
	Chatter
	io.Closer
}
