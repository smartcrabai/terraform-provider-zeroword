variable "google_client_secret" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "zeroword_connection" "google" {
  environment_id     = zeroword_environment.staging.id
  social_provider    = "google"
  mode               = "byo"
  provider_key       = "google"
  provider_client_id = "1234.apps.googleusercontent.com"
  # Write-only: sent to Zeroword, never stored in state.
  provider_client_secret_wo = var.google_client_secret
  # Bump to send a rotated secret.
  provider_client_secret_wo_version = 1
  scopes                            = ["openid", "email", "profile"]
}
