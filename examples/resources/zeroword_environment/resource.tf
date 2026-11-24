resource "zeroword_environment" "staging" {
  project_id = zeroword_project.app.id
  type       = "test"
  name       = "app-staging"
}
