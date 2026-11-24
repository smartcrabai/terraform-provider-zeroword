resource "zeroword_project" "app" {
  workspace_id = data.zeroword_workspace.this.id
  name         = "app"
}
