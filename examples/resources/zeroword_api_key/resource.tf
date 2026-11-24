resource "zeroword_api_key" "deletion_job" {
  workspace_id   = data.zeroword_workspace.this.id
  environment_id = zeroword_environment.staging.id
  name           = "account deletion job"
  scopes         = ["users:read", "users:delete"]
}

output "deletion_job_api_key" {
  value     = zeroword_api_key.deletion_job.secret
  sensitive = true
}
