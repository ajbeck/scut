package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

func TestAWSProxyConfigUsesEndpointAndEnvironmentFallbacks(t *testing.T) {
	cfg := awsProxyCmd{
		Endpoint: "https://service.example.com/mcp",
		Profiles: []string{
			"default",
			"dev",
		},
		CaBundle: new("/tmp/company-ca.pem"),
		Metadata: map[string]string{
			"team":       "platform",
			"AWS_REGION": "us-west-2",
		},
		AllowEmptyTools:  new(true),
		LazyConnect:      new(true),
		ReadOnly:         new(true),
		LogLevel:         new("DEBUG"),
		Retries:          new(3),
		Timeout:          new(10.5),
		ConnectTimeout:   new(2.0),
		ReadTimeout:      new(3.0),
		WriteTimeout:     new(4.0),
		ToolTimeout:      new(5.0),
		DisableTelemetry: new(true),
		OptionalAuth:     new(true),
	}.config(lookupEnv(map[string]string{"AWS_REGION": "eu-west-1"}))

	if cfg.Endpoint == nil || *cfg.Endpoint != "https://service.example.com/mcp" {
		t.Fatalf("Endpoint = %#v", cfg.Endpoint)
	}
	if cfg.Service == nil || *cfg.Service != "service" {
		t.Fatalf("Service = %#v", cfg.Service)
	}
	if cfg.Region == nil || *cfg.Region != "eu-west-1" {
		t.Fatalf("Region = %#v", cfg.Region)
	}
	if cfg.Profiles == nil || strings.Join(*cfg.Profiles, ",") != "default,dev" {
		t.Fatalf("Profiles = %#v", cfg.Profiles)
	}
	if cfg.CaBundle == nil || *cfg.CaBundle != "/tmp/company-ca.pem" {
		t.Fatalf("CaBundle = %#v", cfg.CaBundle)
	}
	if cfg.Metadata == nil || (*cfg.Metadata)["team"] != "platform" || (*cfg.Metadata)["AWS_REGION"] != "us-west-2" {
		t.Fatalf("Metadata = %#v", cfg.Metadata)
	}
	if cfg.AllowEmptyTools == nil || !*cfg.AllowEmptyTools || cfg.LazyConnect == nil || !*cfg.LazyConnect {
		t.Fatalf("expected allow empty tools and lazy connect: %+v", cfg)
	}
	if cfg.ReadOnly == nil || !*cfg.ReadOnly || cfg.LogLevel == nil || *cfg.LogLevel != "DEBUG" || cfg.Retries == nil || *cfg.Retries != 3 {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.Timeout == nil || *cfg.Timeout != 10500*time.Millisecond {
		t.Fatalf("Timeout = %#v", cfg.Timeout)
	}
	if cfg.ConnectTimeout == nil || *cfg.ConnectTimeout != 2*time.Second || cfg.ReadTimeout == nil || *cfg.ReadTimeout != 3*time.Second ||
		cfg.WriteTimeout == nil || *cfg.WriteTimeout != 4*time.Second || cfg.ToolTimeout == nil || *cfg.ToolTimeout != 5*time.Second {
		t.Fatalf("unexpected timeouts: %+v", cfg)
	}
	if cfg.DisableTelemetry == nil || !*cfg.DisableTelemetry || cfg.OptionalAuth == nil || !*cfg.OptionalAuth {
		t.Fatalf("expected disable telemetry and optional auth: %+v", cfg)
	}
}

func TestAWSProxyConfigInfersServiceAndRegionFromAWSAPIEndpoint(t *testing.T) {
	cfg := awsProxyCmd{
		Endpoint: "https://aws-mcp.us-east-1.api.aws/mcp",
	}.config(lookupEnv(nil))

	if cfg.Service == nil || *cfg.Service != "aws-mcp" {
		t.Fatalf("Service = %#v", cfg.Service)
	}
	if cfg.Region == nil || *cfg.Region != "us-east-1" {
		t.Fatalf("Region = %#v", cfg.Region)
	}
}

func TestAWSProxyConfigInfersBedrockAgentCoreEndpoint(t *testing.T) {
	cfg := awsProxyCmd{
		Endpoint: "https://runtime.bedrock-agentcore.us-west-2.amazonaws.com/mcp",
	}.config(lookupEnv(nil))

	if cfg.Service == nil || *cfg.Service != "bedrock-agentcore" {
		t.Fatalf("Service = %#v", cfg.Service)
	}
	if cfg.Region == nil || *cfg.Region != "us-west-2" {
		t.Fatalf("Region = %#v", cfg.Region)
	}
}

func TestAWSProxyConfigDedupesProfiles(t *testing.T) {
	cfg := awsProxyCmd{
		Endpoint: "https://service.us-east-1.api.aws/mcp",
		Profiles: []string{
			"default",
			"dev",
			"default",
			"",
		},
	}.config(lookupEnv(nil))

	if cfg.Profiles == nil || strings.Join(*cfg.Profiles, ",") != "default,dev" {
		t.Fatalf("Profiles = %#v", cfg.Profiles)
	}
}

func TestAWSProxyConfigLeavesOmittedOptionalValuesUnset(t *testing.T) {
	cfg := awsProxyCmd{
		Endpoint: "https://service.us-east-1.api.aws/mcp",
	}.config(lookupEnv(nil))

	if cfg.CaBundle != nil {
		t.Fatalf("CaBundle = %#v, want nil", cfg.CaBundle)
	}
	if cfg.Metadata != nil {
		t.Fatalf("Metadata = %#v, want nil", cfg.Metadata)
	}
	if cfg.Profiles != nil {
		t.Fatalf("Profiles = %#v, want nil", cfg.Profiles)
	}
	if cfg.AllowEmptyTools != nil || cfg.LazyConnect != nil || cfg.ReadOnly != nil || cfg.Retries != nil || cfg.Timeout != nil || cfg.OptionalAuth != nil {
		t.Fatalf("optional defaults were unexpectedly set: %+v", cfg)
	}
}

func TestAWSProxyAuthModesAreMutuallyExclusive(t *testing.T) {
	parser := kong.Must(&awsProxyCmd{})
	_, err := parser.Parse([]string{"https://service.us-east-1.api.aws/mcp", "--skip-auth", "--optional-auth"})
	if err == nil || !strings.Contains(err.Error(), "--skip-auth and --optional-auth can't be used together") {
		t.Fatalf("Parse() error = %v", err)
	}
}

func lookupEnv(values map[string]string) lookupEnvFunc {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
