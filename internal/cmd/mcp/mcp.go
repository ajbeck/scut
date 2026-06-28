// Package mcp implements MCP utility commands.
package mcp

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	awsproxy "github.com/ajbeck/go-aws-mcp-proxy/proxy"

	"github.com/ajbeck/scut/internal/version"
)

// Cmd is the Kong command group for "scut mcp".
type Cmd struct {
	AWSProxy awsProxyCmd `cmd:"aws-proxy" help:"Run an MCP stdio proxy for SigV4-protected AWS MCP endpoints."`
}

type awsProxyCmd struct {
	Endpoint string `arg:"" help:"SigV4 MCP endpoint URL."`

	Service  *string  `help:"AWS service name for SigV4 signing. Inferred from endpoint when omitted."`
	Profiles []string `name:"profile" env:"AWS_MCP_PROXY_PROFILES,AWS_PROFILE" help:"AWS profile(s) to use. First profile is the default." sep:" " placeholder:"PROFILE"`
	Region   *string  `help:"AWS region to sign. Inferred from endpoint or AWS_REGION when omitted."`
	CaBundle *string  `name:"ca-bundle" env:"AWS_CA_BUNDLE" help:"Path to a PEM certificate bundle to trust in addition to the system roots." placeholder:"PATH"`

	Metadata map[string]string `help:"Metadata to inject into MCP requests as key=value pairs." mapsep:"none" placeholder:"KEY=VALUE"`

	ReadOnly *bool `name:"read-only" help:"Disable tools that do not advertise readOnlyHint=true."`

	LogLevel *string `name:"log-level" enum:"DEBUG,INFO,WARNING,ERROR,CRITICAL" help:"Set the logging level."`
	Retries  *int    `help:"Number of retries when calling endpoint MCP. Defaults to 3; 0 disables retries."`

	Timeout        *float64 `help:"Total timeout in seconds when connecting to endpoint."`
	ConnectTimeout *float64 `name:"connect-timeout" help:"Connection timeout in seconds."`
	ReadTimeout    *float64 `name:"read-timeout" help:"Read timeout in seconds."`
	WriteTimeout   *float64 `name:"write-timeout" help:"Write timeout in seconds."`
	ToolTimeout    *float64 `name:"tool-timeout" help:"Maximum seconds a tool call may take before cancellation."`

	DisableTelemetry *bool `name:"disable-telemetry" help:"Disable client telemetry in outbound user-agent data."`
	SkipAuth         *bool `name:"skip-auth" help:"Send unsigned requests when AWS credentials are unavailable."`
}

type runProxyFunc func(context.Context, awsproxy.Config, awsproxy.RunOptions) error

var runAWSProxy runProxyFunc = awsproxy.Run

func (c *awsProxyCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runAWSProxy(ctx, c.config(os.LookupEnv), awsproxy.RunOptions{
		Logger:  newLogger(valueOr(c.LogLevel, "ERROR"), os.Stderr),
		Version: version.String(),
	})
}

type lookupEnvFunc func(string) (string, bool)

func (c awsProxyCmd) config(lookupEnv lookupEnvFunc) awsproxy.Config {
	if lookupEnv == nil {
		lookupEnv = os.LookupEnv
	}

	endpoint := c.Endpoint
	endpointService, endpointRegion := serviceNameAndRegionFromEndpoint(endpoint)
	service := c.Service
	if service == nil && endpointService != "" {
		service = new(endpointService)
	}
	region := c.Region
	if region == nil && endpointRegion != "" {
		region = new(endpointRegion)
	}
	if region == nil {
		if value, ok := lookupEnv("AWS_REGION"); ok {
			region = new(value)
		}
	}

	cfg := awsproxy.Config{
		Endpoint:         &endpoint,
		Service:          service,
		Region:           region,
		CaBundle:         c.CaBundle,
		ReadOnly:         c.ReadOnly,
		LogLevel:         c.LogLevel,
		Retries:          c.Retries,
		Timeout:          seconds(c.Timeout),
		ConnectTimeout:   seconds(c.ConnectTimeout),
		ReadTimeout:      seconds(c.ReadTimeout),
		WriteTimeout:     seconds(c.WriteTimeout),
		ToolTimeout:      seconds(c.ToolTimeout),
		DisableTelemetry: c.DisableTelemetry,
		SkipAuth:         c.SkipAuth,
	}
	profiles := dedupe(c.Profiles)
	if len(profiles) > 0 {
		cfg.Profiles = &profiles
	}
	if len(c.Metadata) > 0 {
		metadata := c.Metadata
		cfg.Metadata = &metadata
	}
	return cfg
}

func valueOr[T any](ptr *T, fallback T) T {
	if ptr == nil {
		return fallback
	}
	return *ptr
}

func seconds(value *float64) *time.Duration {
	if value == nil {
		return nil
	}
	return new(time.Duration(*value * float64(time.Second)))
}

func dedupe(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func serviceNameAndRegionFromEndpoint(endpoint string) (string, string) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", ""
	}
	host := parsed.Hostname()
	if host == "" {
		return "", ""
	}

	parts := strings.Split(host, ".")
	if len(parts) >= 4 {
		tail := parts[len(parts)-4:]
		if tail[0] == "bedrock-agentcore" && tail[2] == "amazonaws" && tail[3] == "com" {
			return "bedrock-agentcore", tail[1]
		}
	}
	if len(parts) == 4 && parts[2] == "api" && parts[3] == "aws" {
		return parts[0], parts[1]
	}
	if parts[0] != "" {
		return parts[0], ""
	}
	return "", ""
}

func newLogger(levelName string, w io.Writer) *slog.Logger {
	level := slog.LevelError
	switch strings.ToUpper(levelName) {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARNING":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	case "CRITICAL":
		level = slog.LevelError + 4
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
