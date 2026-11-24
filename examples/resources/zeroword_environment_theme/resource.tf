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
    colorScheme = "system"
    darkAppearance = {
      primaryColor    = "#f9fafb"
      backgroundColor = "#111827"
      textColor       = "#f9fafb"
    }
    methods       = ["passkey", "email_otp"]
    defaultLocale = "ja"
  })
}
