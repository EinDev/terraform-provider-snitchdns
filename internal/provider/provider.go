// Package provider implements the SnitchDNS Terraform provider.
package provider

import (
	"context"
	"os"

	"snitchdns-tf/internal/client"
	"snitchdns-tf/internal/testcontainer"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure SnitchDNSProvider satisfies various provider interfaces.
var _ provider.Provider = &SnitchDNSProvider{}

// SnitchDNSProvider defines the provider implementation.
type SnitchDNSProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance testing.
	version   string
	container *testcontainer.SnitchDNSContainer
}

// SnitchDNSProviderModel describes the provider data model.
type SnitchDNSProviderModel struct {
	APIUrl string `tfsdk:"api_url"`
	APIKey string `tfsdk:"api_key"`
}

func (p *SnitchDNSProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "snitchdns"
	resp.Version = p.version
}

func (p *SnitchDNSProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provider for managing SnitchDNS resources. Configure with API endpoint and authentication key.",
		Attributes: map[string]schema.Attribute{
			"api_url": schema.StringAttribute{
				MarkdownDescription: "SnitchDNS API URL. Can also be set via SNITCHDNS_API_URL environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "SnitchDNS API Key for authentication. Can also be set via SNITCHDNS_API_KEY environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

// Configure prepares the SnitchDNS API client for data sources and resources.
func (p *SnitchDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data SnitchDNSProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Use environment variables as fallback
	apiURL := data.APIUrl
	if apiURL == "" {
		apiURL = os.Getenv("SNITCHDNS_API_URL")
	}

	apiKey := data.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("SNITCHDNS_API_KEY")
	}

	// Validate required configuration
	if apiURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_url"),
			"Missing API URL",
			"The provider cannot create the SnitchDNS API client as there is a missing or empty value for the API URL. "+
				"Set the api_url value in the provider configuration or use the SNITCHDNS_API_URL environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing API Key",
			"The provider cannot create the SnitchDNS API client as there is a missing or empty value for the API key. "+
				"Set the api_key value in the provider configuration or use the SNITCHDNS_API_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Configuring SnitchDNS client", map[string]any{
		"api_url": apiURL,
	})

	// Create API client
	client := client.NewClient(apiURL, apiKey)

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *SnitchDNSProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewZoneResource,
		NewRecordResource,
	}
}

func (p *SnitchDNSProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func New(version string, container *testcontainer.SnitchDNSContainer) func() provider.Provider {
	return func() provider.Provider {
		return &SnitchDNSProvider{
			version:   version,
			container: container,
		}
	}
}
