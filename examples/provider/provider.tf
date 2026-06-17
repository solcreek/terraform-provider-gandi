provider "gandi" {
  # The Personal Access Token is read from the GANDI_PAT environment variable
  # by default; it can also be set explicitly here.
  # personal_access_token = "..."
  timeout_seconds = 30

  # Target the Gandi sandbox instead of production. Requires a separate sandbox
  # account and a sandbox-specific PAT. Equivalent to
  # api_url = "https://api.sandbox.gandi.net".
  # sandbox = true
}
