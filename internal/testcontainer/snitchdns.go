// Package testcontainer provides Docker container setup for SnitchDNS testing.
package testcontainer

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Default configuration values for SnitchDNS test containers.
const (
	// DefaultUsername is the default admin username for test containers.
	DefaultUsername = "testadmin"
	DefaultPassword = "password123"
	HTTPPort        = "80/tcp"
	DNSPort         = "2024/udp"
)

// SnitchDNSContainer represents a SnitchDNS test container
type SnitchDNSContainer struct {
	Container testcontainers.Container
	HTTPHost  string
	APIKey    string
}

// SnitchDNSContainerRequest configures the SnitchDNS container
type SnitchDNSContainerRequest struct {
	// DockerfilePath is the path to the directory containing the Dockerfile
	// Defaults to "./testcontainer"
	DockerfilePath string

	// ExposePorts determines if container ports should be exposed to host
	// Set to false in CI environments where network access is direct
	ExposePorts bool
}

// NewSnitchDNSContainer creates and starts a new SnitchDNS container
func NewSnitchDNSContainer(ctx context.Context, req SnitchDNSContainerRequest) (*SnitchDNSContainer, error) {
	if req.DockerfilePath == "" {
		req.DockerfilePath = "../../testcontainer"
	}

	// Convert to absolute path from the current package directory
	absPath, err := filepath.Abs(req.DockerfilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dockerfile path: %w", err)
	}

	// Try to use pre-built image first (for CI), fall back to building from Dockerfile
	containerReq := testcontainers.ContainerRequest{
		Image: "snitchdns-test:latest",
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    absPath,
			Dockerfile: "Dockerfile",
			KeepImage:  true, // Keep image for faster subsequent runs
		},
		ExposedPorts: []string{HTTPPort, DNSPort},
		WaitingFor: wait.ForAll(
			wait.ForLog("Starting Flask web application on port 80").WithStartupTimeout(120*time.Second),
			wait.ForHTTP("/").WithPort(HTTPPort).WithStartupTimeout(120*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: containerReq,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get the host and port
	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, HTTPPort)
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	httpHost := fmt.Sprintf("http://%s:%s", host, port.Port())

	// Extract API key from container logs
	apiKey, err := extractAPIKey(ctx, container)
	if err != nil {
		return nil, fmt.Errorf("failed to extract API key: %w", err)
	}

	return &SnitchDNSContainer{
		Container: container,
		HTTPHost:  httpHost,
		APIKey:    apiKey,
	}, nil
}

// extractAPIKey reads the API key from the container
func extractAPIKey(ctx context.Context, container testcontainers.Container) (string, error) {
	// Read the API key file from container
	code, reader, err := container.Exec(ctx, []string{"cat", "/tmp/apikey.txt"})
	if err != nil {
		return "", fmt.Errorf("failed to execute cat command: %w", err)
	}

	if code != 0 {
		return "", fmt.Errorf("cat command failed with exit code %d", code)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read apikey output: %w", err)
	}

	// Debug: print the raw output to understand what we're getting
	rawOutput := string(output)

	// Parse API_KEY=xxx format
	line := strings.TrimSpace(rawOutput)

	// Find the API_KEY line if there are multiple lines
	lines := strings.Split(line, "\n")
	var apiKeyLine string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		// Find "API_KEY=" anywhere in the line (not just at start)
		if idx := strings.Index(l, "API_KEY="); idx >= 0 {
			apiKeyLine = l[idx:] // Take from API_KEY= onwards
			break
		}
	}

	if apiKeyLine == "" {
		return "", fmt.Errorf("API_KEY not found. Raw output (hex): % x\nString output: %q", rawOutput, rawOutput)
	}

	parts := strings.SplitN(apiKeyLine, "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid apikey format: %q (hex: % x)", apiKeyLine, apiKeyLine)
	}

	apiKey := strings.TrimSpace(parts[1])
	if apiKey == "" {
		return "", fmt.Errorf("empty API key")
	}

	return apiKey, nil
}

// Terminate stops and removes the container
func (c *SnitchDNSContainer) Terminate(ctx context.Context) error {
	if c.Container != nil {
		return c.Container.Terminate(ctx)
	}
	return nil
}

// GetAPIEndpoint returns the full API endpoint URL
func (c *SnitchDNSContainer) GetAPIEndpoint() string {
	return c.HTTPHost + "/api/v1"
}

// GetDNSPort returns the mapped DNS port
func (c *SnitchDNSContainer) GetDNSPort(ctx context.Context) (string, error) {
	port, err := c.Container.MappedPort(ctx, DNSPort)
	if err != nil {
		return "", err
	}
	return port.Port(), nil
}

// Logs returns the container logs
func (c *SnitchDNSContainer) Logs(ctx context.Context) (string, error) {
	reader, err := c.Container.Logs(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(logs), nil
}
