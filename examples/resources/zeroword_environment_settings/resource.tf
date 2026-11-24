resource "zeroword_environment_settings" "staging" {
  environment_id        = zeroword_environment.staging.id
  impersonation_enabled = false
}
