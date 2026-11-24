resource "zeroword_domain" "login" {
  environment_id = zeroword_environment.staging.id
  hostname       = "login.example.com"
}

# Publish the ownership TXT record, e.g. with Cloudflare.
resource "cloudflare_dns_record" "zeroword_login" {
  zone_id = var.cloudflare_zone_id
  name    = zeroword_domain.login.verification_record_name
  type    = "TXT"
  content = zeroword_domain.login.verification_record_value
  ttl     = 300
}
