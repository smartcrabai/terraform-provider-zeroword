terraform {
  required_providers {
    zeroword = {
      source = "smartcrabai/zeroword"
    }
  }
}

# The API key is read from ZEROWORD_API_KEY when api_key is omitted.
provider "zeroword" {
  base_url = "https://api.staging.zeroword.smartcrab.ai/v1"
}
