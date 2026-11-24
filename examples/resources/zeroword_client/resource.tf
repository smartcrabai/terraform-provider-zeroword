resource "zeroword_client" "web" {
  environment_id             = zeroword_environment.staging.id
  client_type                = "web_spa"
  name                       = "web"
  token_endpoint_auth_method = "none"
  redirect_uris              = ["https://app.example.com/callback"]
  post_logout_redirect_uris  = ["https://app.example.com/"]
  allowed_origins            = ["https://app.example.com"]
  allowed_scopes             = ["openid", "email", "profile", "offline_access"]
  allowed_resources          = ["https://api.example.com"]
}
