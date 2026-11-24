resource "zeroword_webhook" "events" {
  environment_id = zeroword_environment.staging.id
  url            = "https://api.example.com/zeroword/webhook"
  event_types    = ["user.created", "user.deleted"]
}
