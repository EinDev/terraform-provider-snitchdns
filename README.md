# SnitchDNS Terraform Provider

A Terraform provider for managing [SnitchDNS](https://github.com/sadreck/SnitchDNS) zones and DNS records via the SnitchDNS API.

Manage your DNS infrastructure as code with full support for zones, records, tags, and advanced features like conditional responses, catch-all zones, and regex pattern matching.

## Features

- **Zone Management**: Create, update, and delete DNS zones with advanced options
  - Catch-all zones for wildcard subdomain matching
  - DNS forwarding to upstream resolvers
  - Regex pattern matching for flexible domain handling
  - Tag-based organization
- **Record Management**: Full support for all standard DNS record types
  - A, AAAA, CNAME, MX, TXT, NS, SRV, CAA, and more (18 types total)
  - Conditional responses for canary deployments and A/B testing
  - Flexible TTL configuration
- **Import Support**: Import existing zones and records into Terraform state
- **Defensive Operations**: Automatic detection and cleanup of externally deleted resources
- **Comprehensive Validation**: Client-side validation for common errors
- **Environment Variables**: Secure configuration via environment variables

## Quick Start

### Installation

Add the provider to your Terraform configuration:

```terraform
terraform {
  required_providers {
    snitchdns = {
      source  = "EinDev/snitchdns"
      version = "~> 1.0"
    }
  }
}
```

### Basic Usage

```terraform
provider "snitchdns" {
  api_url = "http://localhost:8000"
  api_key = "your-api-key-here"
}

# Create a DNS zone
resource "snitchdns_zone" "example" {
  domain     = "example.com"
  active     = true
  catch_all  = false
  forwarding = false
  regex      = false
  tags       = ["production"]
}

# Create an A record
resource "snitchdns_record" "www" {
  zone_id = snitchdns_zone.example.id
  active  = true
  cls     = "IN"
  type    = "A"
  ttl     = 3600

  data = {
    address = "192.168.1.100"
  }
}
```

### Using Environment Variables

For better security, use environment variables instead of hardcoding credentials:

```bash
export SNITCHDNS_API_URL="http://localhost:8000"
export SNITCHDNS_API_KEY="your-api-key"
```

```terraform
provider "snitchdns" {
  # Configuration from environment variables
}
```

## Documentation

- **[Provider Configuration](docs/index.md)** - Setup and authentication
- **[Zone Resource](docs/resources/zone.md)** - Managing DNS zones
- **[Record Resource](docs/resources/record.md)** - Managing DNS records

## Examples

The [examples/](examples/) directory contains working Terraform configurations:

- **[basic/](examples/basic/)** - Simple zone and record creation
- **[complete/](examples/complete/)** - Full web infrastructure with mail, CAA, and more
- **[advanced/](examples/advanced/)** - Multi-environment, load balancing, canary deployments

### Example: Complete Web Infrastructure

```terraform
resource "snitchdns_zone" "production" {
  domain     = "example.com"
  active     = true
  catch_all  = false
  forwarding = false
  regex      = false
  tags       = ["production", "web"]
}

# Root domain
resource "snitchdns_record" "root" {
  zone_id = snitchdns_zone.production.id
  active  = true
  cls     = "IN"
  type    = "A"
  ttl     = 3600
  data    = { address = "192.168.1.100" }
}

# WWW subdomain
resource "snitchdns_record" "www" {
  zone_id = snitchdns_zone.production.id
  active  = true
  cls     = "IN"
  type    = "CNAME"
  ttl     = 3600
  data    = { name = "example.com." }
}

# Mail server
resource "snitchdns_record" "mx" {
  zone_id = snitchdns_zone.production.id
  active  = true
  cls     = "IN"
  type    = "MX"
  ttl     = 3600
  data = {
    priority = "10"
    hostname = "mail.example.com."
  }
}
```

## Requirements

- **Terraform**: >= 1.0
- **Go**: >= 1.24 (for development)
- **SnitchDNS**: Server with API access
- **Docker**: For running tests (development only)
- **golangci-lint**: v2.7.2 (for development)

## Obtaining an API Key

1. Log in to your SnitchDNS web interface
2. Navigate to **Settings > API**
3. Click **Generate New API Key**
4. Copy the key and use it in your provider configuration

## Importing Existing Resources

You can import existing zones and records into Terraform:

```bash
# Import a zone
terraform import snitchdns_zone.example 123

# Import a record (format: zone_id:record_id)
terraform import snitchdns_record.www 123:456
```

## Development

This provider is developed using Test-Driven Development (TDD) with testcontainers for integration testing.

### Prerequisites

- Go 1.24 or later
- Docker (for running testcontainers)
- golangci-lint v2.7.2 (install with `make install-tools`)
- Make (optional, for convenience commands)

### Running Tests

```bash
# Run all tests
make test

# Run only unit tests (fast, no containers)
make test-unit

# Run integration tests (starts testcontainer)
make test-integration

# Run with verbose output
go test -v ./...

# Check test coverage
go test -cover ./...
```

### Building the Provider

```bash
# Build the provider binary
make build

# Install locally for testing
make install
```

### Test Container

The project includes a testcontainer that provides a fully functional SnitchDNS instance:

- **Image**: Built from official SnitchDNS repository
- **Database**: SQLite (for test simplicity)
- **Default User**: `testadmin` / `password123`
- **API Key**: Automatically generated and exposed
- **Ports**: HTTP (80), HTTPS (443), DNS (2024/udp)

Example usage:

```go
ctx := context.Background()
container, err := testcontainer.NewSnitchDNSContainer(ctx,
    testcontainer.SnitchDNSContainerRequest{
        ExposePorts: true,
    })
if err != nil {
    t.Fatal(err)
}
defer container.Terminate(ctx)

apiEndpoint := container.GetAPIEndpoint()
apiKey := container.APIKey
```

### Useful Commands

```bash
# Format code
make fmt

# Run linters
make lint

# Run go vet
make vet

# Clean build artifacts
make clean

# Run acceptance tests (requires TF_ACC=1)
TF_ACC=1 go test ./... -v
```

### Development Workflow

1. Write a failing test that describes the desired functionality
2. Run the test to verify it fails
3. Implement the feature
4. Run the test again to verify it passes
5. Refactor if needed while keeping tests green

## CI/CD

The project uses GitHub Actions for continuous integration:

- Runs on every push and pull request
- Executes all tests including integration tests
- Checks code formatting and linting with golangci-lint v2.7.2
- Uploads coverage reports to Codecov
- Supports Go 1.24+

See [`.github/workflows/`](.github/workflows/) for workflow configurations.

## Project Structure

```
.
├── docs/                      # Documentation
│   ├── index.md              # Provider documentation
│   └── resources/            # Resource documentation
│       ├── zone.md
│       └── record.md
├── examples/                  # Usage examples
│   ├── basic/
│   ├── complete/
│   └── advanced/
├── internal/
│   ├── client/               # API client
│   ├── provider/             # Terraform provider implementation
│   │   ├── resource_zone.go
│   │   └── resource_record.go
│   └── testcontainer/        # Test container setup
├── testcontainer/            # Docker setup for tests
│   ├── Dockerfile
│   └── entrypoint.sh
├── .github/
│   └── workflows/            # CI/CD pipelines
├── Makefile                  # Development commands
├── CHANGELOG.md              # Version history
└── README.md                 # This file
```

## Best Practices

### Security

- **Never commit API keys** to version control
- Use environment variables for credentials
- Keep `terraform.tfvars` in `.gitignore`
- Use sensitive variable types in Terraform modules

### DNS Configuration

- Use trailing dots (`.`) for FQDNs in record data
- Set appropriate TTL values based on change frequency
- Consider caching implications when choosing TTL
- Test DNS changes in dev/staging before production

### Terraform Workflow

- Use `terraform plan` before `apply`
- Enable remote state for team collaboration
- Use workspaces for environment separation
- Tag resources consistently for organization

## Troubleshooting

### Provider Not Found

Ensure the provider is properly configured in your `terraform` block and run:

```bash
terraform init
```

### Authentication Errors

Verify your API key and URL:

```bash
# Test API access
curl -H "X-Api-Key: your-api-key" http://localhost:8000/api/v1/zones
```

### Resource Not Found After External Deletion

This is expected behavior. Terraform will detect the external deletion and remove the resource from state during the next `plan` or `apply`.

### Tests Timeout

Increase the test timeout:

```bash
go test -timeout 10m ./...
```

## Contributing

Contributions are welcome! Please ensure:

- All tests pass: `make test`
- Code is formatted: `make fmt`
- Linters pass: `make lint`
- New features include tests and documentation
- Follow the existing code style

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Ensure all tests pass
5. Update documentation
6. Submit a pull request

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and release notes.

## License

TBD

## Support

- **Issues**: [GitHub Issues](https://github.com/EinDev/snitchdns-tf/issues)
- **Discussions**: [GitHub Discussions](https://github.com/EinDev/snitchdns-tf/discussions)
- **SnitchDNS**: [Official Documentation](https://github.com/sadreck/SnitchDNS)

## Acknowledgments

- [SnitchDNS](https://github.com/sadreck/SnitchDNS) - The DNS server this provider manages
- [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) - The framework used to build this provider
- [Testcontainers](https://golang.testcontainers.org/) - Integration testing framework
