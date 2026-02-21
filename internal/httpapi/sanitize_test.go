package httpapi

import (
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/config"
)

func TestSanitizeConfigRedactsDockerRuntimeSettings(t *testing.T) {
	cfg := config.Default()
	cfg.Provisioning.Docker.Endpoint = "unix:///var/run/docker.sock"
	cfg.Provisioning.Docker.HostAddress = "10.0.0.1"
	cfg.Provisioning.Docker.NetworkName = "private-net"

	sanitized := sanitizeConfig(cfg, true, true)

	if sanitized.Provisioning.Docker.Endpoint != "[redacted]" {
		t.Fatalf("endpoint = %q, want [redacted]", sanitized.Provisioning.Docker.Endpoint)
	}
	if sanitized.Provisioning.Docker.HostAddress != "[redacted]" {
		t.Fatalf("host_address = %q, want [redacted]", sanitized.Provisioning.Docker.HostAddress)
	}
	if sanitized.Provisioning.Docker.NetworkName != "[redacted]" {
		t.Fatalf("network_name = %q, want [redacted]", sanitized.Provisioning.Docker.NetworkName)
	}
}
