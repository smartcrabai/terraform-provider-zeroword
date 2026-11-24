package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var protoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"zeroword": providerserver.NewProtocol6WithError(New("test")()),
}

// setup runs the tests against the in-memory fake API, or against a real
// Management API when TF_ACC=1 (ZEROWORD_API_KEY, ZEROWORD_BASE_URL and
// ZEROWORD_ACC_WORKSPACE_ID must then be set). fake is nil in acceptance mode.
func setup(t *testing.T) (fake *fakeAPI, workspaceID string) {
	t.Helper()
	if os.Getenv("TF_ACC") == "1" {
		workspaceID = os.Getenv("ZEROWORD_ACC_WORKSPACE_ID")
		if workspaceID == "" || os.Getenv("ZEROWORD_API_KEY") == "" || os.Getenv("ZEROWORD_BASE_URL") == "" {
			t.Fatal("TF_ACC=1 requires ZEROWORD_API_KEY, ZEROWORD_BASE_URL and ZEROWORD_ACC_WORKSPACE_ID")
		}
		return nil, workspaceID
	}
	return newFakeAPI(t), fakeWorkspaceID
}

func testCase(t *testing.T, steps ...resource.TestStep) resource.TestCase {
	return resource.TestCase{
		IsUnitTest:               os.Getenv("TF_ACC") != "1",
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		// Write-only attributes need Terraform 1.11.
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps:                  steps,
	}
}

func baseConfig(workspaceID, suffix string) string {
	return fmt.Sprintf(`
data "zeroword_workspace" "this" {
  id = %[1]q
}

resource "zeroword_project" "app" {
  workspace_id = data.zeroword_workspace.this.id
  name         = "tfacc-%[2]s"
}

resource "zeroword_environment" "staging" {
  project_id = zeroword_project.app.id
  type       = "test"
  name       = "tfacc-staging-%[2]s"
}
`, workspaceID, suffix)
}

func TestProjectEnvironmentAndSettings(t *testing.T) {
	_, workspaceID := setup(t)
	config := func(name string, impersonation bool) string {
		return baseConfig(workspaceID, name) + fmt.Sprintf(`
resource "zeroword_environment_settings" "staging" {
  environment_id        = zeroword_environment.staging.id
  impersonation_enabled = %t
}
`, impersonation)
	}
	resource.Test(t, testCase(t,
		resource.TestStep{
			Config: config("one", false),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.zeroword_workspace.this", "id", workspaceID),
				resource.TestCheckResourceAttrPair("zeroword_project.app", "workspace_id", "data.zeroword_workspace.this", "id"),
				resource.TestCheckResourceAttrSet("zeroword_project.app", "slug"),
				resource.TestCheckResourceAttr("zeroword_environment.staging", "status", "active"),
				resource.TestMatchResourceAttr("zeroword_environment.staging", "issuer", regexp.MustCompile(`^https://`)),
				resource.TestCheckResourceAttr("zeroword_environment_settings.staging", "impersonation_enabled", "false"),
			),
		},
		resource.TestStep{
			// Renames update in place; the IDs stay.
			Config: config("two", true),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("zeroword_project.app", plancheck.ResourceActionUpdate),
				plancheck.ExpectResourceAction("zeroword_environment.staging", plancheck.ResourceActionUpdate),
			}},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("zeroword_project.app", "name", "tfacc-two"),
				resource.TestCheckResourceAttr("zeroword_environment_settings.staging", "impersonation_enabled", "true"),
			),
		},
		resource.TestStep{ResourceName: "zeroword_project.app", ImportState: true, ImportStateVerify: true},
		resource.TestStep{ResourceName: "zeroword_environment.staging", ImportState: true, ImportStateVerify: true},
		resource.TestStep{
			ResourceName:                         "zeroword_environment_settings.staging",
			ImportState:                          true,
			ImportStateVerify:                    true,
			ImportStateIdFunc:                    attribute("zeroword_environment.staging", "id"),
			ImportStateVerifyIdentifierAttribute: "environment_id",
		},
	))
}

func TestClientsThemeAndSecrets(t *testing.T) {
	_, workspaceID := setup(t)
	config := func(redirect, methods string) string {
		return baseConfig(workspaceID, "clients") + fmt.Sprintf(`
resource "zeroword_client" "spa" {
  environment_id             = zeroword_environment.staging.id
  client_type                = "web_spa"
  name                       = "SPA"
  token_endpoint_auth_method = "none"
  redirect_uris              = [%[1]q]
  allowed_origins            = ["https://app.example.com"]
  allowed_scopes             = ["openid", "email", "offline_access"]
}

resource "zeroword_client" "bff" {
  environment_id             = zeroword_environment.staging.id
  client_type                = "web_bff"
  name                       = "BFF"
  token_endpoint_auth_method = "client_secret_basic"
  status                     = "disabled"
}

resource "zeroword_environment_theme" "staging" {
  environment_id = zeroword_environment.staging.id
  theme = jsonencode({
    appearance = {
      primaryColor    = "#111827"
      backgroundColor = "#ffffff"
      textColor       = "#111827"
      borderRadius    = 12
      fontFamily      = "system"
      density         = "comfortable"
    }
    methods       = %[2]s
    defaultLocale = "ja"
  })
}
`, redirect, methods)
	}
	resource.Test(t, testCase(t,
		resource.TestStep{
			Config: config("https://app.example.com/callback", `["email_otp", "passkey"]`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("zeroword_client.spa", "redirect_uris.#", "1"),
				resource.TestCheckNoResourceAttr("zeroword_client.spa", "client_secret"),
				resource.TestCheckResourceAttrSet("zeroword_client.bff", "client_secret"),
				resource.TestCheckResourceAttr("zeroword_client.bff", "status", "disabled"),
				resource.TestCheckResourceAttrSet("zeroword_environment_theme.staging", "version"),
			),
		},
		resource.TestStep{
			Config: config("https://app.example.com/auth/callback", `["passkey", "email_otp"]`),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("zeroword_client.spa", plancheck.ResourceActionUpdate),
				plancheck.ExpectResourceAction("zeroword_environment_theme.staging", plancheck.ResourceActionUpdate),
				plancheck.ExpectResourceAction("zeroword_client.bff", plancheck.ResourceActionNoop),
			}},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckTypeSetElemAttr("zeroword_client.spa", "redirect_uris.*", "https://app.example.com/auth/callback"),
				// The secret is only returned at creation and must survive updates and refreshes.
				resource.TestCheckResourceAttrSet("zeroword_client.bff", "client_secret"),
			),
		},
		resource.TestStep{
			ResourceName:            "zeroword_client.bff",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"client_secret"},
		},
		resource.TestStep{
			ResourceName:                         "zeroword_environment_theme.staging",
			ImportState:                          true,
			ImportStateVerify:                    true,
			ImportStateIdFunc:                    attribute("zeroword_environment.staging", "id"),
			ImportStateVerifyIdentifierAttribute: "environment_id",
		},
	))
}

func TestAPIKeyReplacementAndRevocation(t *testing.T) {
	fake, workspaceID := setup(t)
	config := func(scopes string) string {
		return baseConfig(workspaceID, "keys") + fmt.Sprintf(`
resource "zeroword_api_key" "deletion_job" {
  workspace_id   = data.zeroword_workspace.this.id
  environment_id = zeroword_environment.staging.id
  name           = "deletion job"
  scopes         = %s
}
`, scopes)
	}
	var firstID, firstSecret string
	steps := []resource.TestStep{
		{
			Config: config(`["users:read"]`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestMatchResourceAttr("zeroword_api_key.deletion_job", "secret", regexp.MustCompile(`^sk_`)),
				resource.TestCheckResourceAttrSet("zeroword_api_key.deletion_job", "prefix"),
				capture("zeroword_api_key.deletion_job", "id", &firstID),
				capture("zeroword_api_key.deletion_job", "secret", &firstSecret),
			),
		},
		{
			// Keys cannot be edited: a scope change revokes and reissues.
			Config: config(`["users:read", "users:delete"]`),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("zeroword_api_key.deletion_job", plancheck.ResourceActionDestroyBeforeCreate),
			}},
			Check: func(s *terraform.State) error {
				key := s.RootModule().Resources["zeroword_api_key.deletion_job"].Primary
				if key.ID == firstID || key.Attributes["secret"] == firstSecret {
					return fmt.Errorf("expected a new key, got the same id or secret")
				}
				return nil
			},
		},
		{
			ResourceName:            "zeroword_api_key.deletion_job",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"secret"},
		},
	}
	if fake != nil {
		// A key revoked outside Terraform is planned for re-creation.
		steps = append(steps, resource.TestStep{
			PreConfig: func() {
				fake.update(newestOf(fake, "mky"), func(o object) { o["revoked_at"] = "2026-11-24T00:00:00.000Z" })
			},
			Config: config(`["users:read", "users:delete"]`),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("zeroword_api_key.deletion_job", plancheck.ResourceActionCreate),
			}},
		})
	}
	resource.Test(t, testCase(t, steps...))
}

func TestConnectionWriteOnlySecret(t *testing.T) {
	fake, workspaceID := setup(t)
	config := func(secret string, version int, issuer string, ownerConfirmed string) string {
		return baseConfig(workspaceID, "connections") + fmt.Sprintf(`
resource "zeroword_connection" "oidc" {
  environment_id                    = zeroword_environment.staging.id
  social_provider                   = "generic_oidc"
  mode                              = "byo"
  provider_key                      = "corp"
  issuer                            = %q
  provider_client_id                = "client-id"
  provider_client_secret_wo         = %q
  provider_client_secret_wo_version = %d
  scopes                            = ["openid", "email"]
  profile_mapping                   = { email = "email" }
  %s
}
`, issuer, secret, version, ownerConfirmed)
	}
	connectionSecret := func(want string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			connection := s.RootModule().Resources["zeroword_connection.oidc"].Primary
			if _, stored := connection.Attributes["provider_client_secret_wo"]; stored {
				return fmt.Errorf("write-only secret was stored in state")
			}
			if fake != nil && fake.connectionSecret(connection.ID) != want {
				return fmt.Errorf("API holds secret %q, want %q", fake.connectionSecret(connection.ID), want)
			}
			return nil
		}
	}
	resource.Test(t, testCase(t,
		resource.TestStep{
			Config: config("first", 1, "https://idp.example.com", "owner_confirmed = true"),
			Check: resource.ComposeAggregateTestCheckFunc(
				connectionSecret("first"),
				resource.TestCheckResourceAttr("zeroword_connection.oidc", "owner_confirmed", "true"),
			),
		},
		resource.TestStep{
			// A new secret without a version bump is not sent.
			Config:           config("ignored", 1, "https://idp.example.com", "owner_confirmed = true"),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			Check:            connectionSecret("first"),
		},
		resource.TestStep{
			Config: config("second", 2, "https://idp.example.com", "owner_confirmed = true"),
			Check:  connectionSecret("second"),
		},
		resource.TestStep{
			// Changing the issuer without configuring owner_confirmed must not re-approve it.
			Config: config("second", 2, "https://idp2.example.com", ""),
			Check:  resource.TestCheckResourceAttr("zeroword_connection.oidc", "owner_confirmed", "false"),
		},
		resource.TestStep{
			ResourceName:            "zeroword_connection.oidc",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"provider_client_secret_wo_version"},
		},
	))
}

func TestDomainAndWebhook(t *testing.T) {
	_, workspaceID := setup(t)
	config := func(events string) string {
		return baseConfig(workspaceID, "hooks") + fmt.Sprintf(`
resource "zeroword_domain" "login" {
  environment_id = zeroword_environment.staging.id
  hostname       = "login.tfacc-zeroword.example"
}

resource "zeroword_webhook" "events" {
  environment_id = zeroword_environment.staging.id
  url            = "https://hooks.example.com/zeroword"
  event_types    = %s
}
`, events)
	}
	resource.Test(t, testCase(t,
		resource.TestStep{
			Config: config(`["user.created"]`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("zeroword_domain.login", "verification_record_name"),
				resource.TestCheckResourceAttr("zeroword_domain.login", "rp_enabled", "false"),
				resource.TestCheckResourceAttrSet("zeroword_webhook.events", "secret"),
			),
		},
		resource.TestStep{
			Config: config(`["user.created", "user.deleted"]`),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("zeroword_webhook.events", plancheck.ResourceActionUpdate),
			}},
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("zeroword_webhook.events", "event_types.#", "2"),
				resource.TestCheckResourceAttrSet("zeroword_webhook.events", "secret"),
			),
		},
		resource.TestStep{ResourceName: "zeroword_domain.login", ImportState: true, ImportStateVerify: true},
		resource.TestStep{
			ResourceName:            "zeroword_webhook.events",
			ImportState:             true,
			ImportStateVerify:       true,
			ImportStateVerifyIgnore: []string{"secret"},
		},
	))
}

func attribute(resourceName, key string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		return s.RootModule().Resources[resourceName].Primary.Attributes[key], nil
	}
}

func capture(resourceName, key string, out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		*out = s.RootModule().Resources[resourceName].Primary.Attributes[key]
		return nil
	}
}

// newestOf returns the newest stored ID with the given prefix.
func newestOf(f *fakeAPI, prefix string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	newest := ""
	for id := range f.objects {
		if len(id) > len(prefix) && id[:len(prefix)] == prefix && id > newest {
			newest = id
		}
	}
	return newest
}
