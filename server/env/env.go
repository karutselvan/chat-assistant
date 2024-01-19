package env

import (
	"context"
	"fmt"
	"os"
)

type Platform int
type DeploymentEnv int

const (
	UNKNOWN Platform = iota
	GOTEST
	IDE
	CLOUDRUN
	COMMANDLINE
)

const (
	DEV DeploymentEnv = iota
	TEST
	STAGING
	PRODUCTION
)

type Environment struct {
	PalmApiKey string
	// Databse hostname
	DatabaseHostname string
	DatabaseUserName string
	DatabasePassword string
	DatabaseDatabase string

	SlackBotOAuthToken string
	SlackClientID      string
	SlackClientSecret  string
	SlackSigningSecret string

	Platform Platform
}

type EnvironmentKeyType int

const EnvironmentKey EnvironmentKeyType = 0

var (
	PresetPlatform Platform
)

func NewEnvironment() (*Environment, error) {
	plat, err := GuessPlatform()
	if err != nil {
		return nil, err
	}
	return NewEnvironmentForPlatform(plat)
}

func NewEnvironmentForPlatform(platform Platform) (*Environment, error) {
	environment := &Environment{
		PalmApiKey:         os.Getenv("PALM_KEY"),
		DatabaseHostname:   os.Getenv("PG_HOSTNAME"),
		DatabaseUserName:   os.Getenv("PG_USERNAME"),
		DatabasePassword:   os.Getenv("PG_PASSWORD"),
		DatabaseDatabase:   os.Getenv("PG_DATABASE"),
		SlackBotOAuthToken: os.Getenv("SLACK_BOT_OAUTH_TOKEN"),
		SlackClientID:      os.Getenv("SLACK_CLIENT_ID"),
		SlackClientSecret:  os.Getenv("SLACK_CLIENT_SECRET"),
		SlackSigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		Platform:           platform,
	}

	switch platform {
	case COMMANDLINE, GOTEST, CLOUDRUN, IDE:
		return environment, nil

	default:
		return nil, fmt.Errorf("invalid environment specified")
	}
}

func FromContext(ctx context.Context) (*Environment, bool) {
	o := ctx.Value(EnvironmentKey)
	e, ok := o.(*Environment)
	return e, ok
}

func NewContext(ctx context.Context, environment *Environment) context.Context {
	rctx := context.WithValue(ctx, EnvironmentKey, environment)
	return rctx
}

func runningOnCloudRun() bool {
	for _, ev := range []string{
		"K_CONFIGURATION",
		"K_REVISION",
		"K_SERVICE",
	} {
		if os.Getenv(ev) == "" {
			return false
		}
	}
	return true
}

func GuessPlatform() (Platform, error) {
	if PresetPlatform != UNKNOWN {
		return PresetPlatform, nil
	}

	if runningOnCloudRun() {
		PresetPlatform = CLOUDRUN
		return PresetPlatform, nil
	}
	return GOTEST, nil
}
