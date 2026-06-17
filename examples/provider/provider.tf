provider "gandi" {
  # The Personal Access Token is read from the GANDI_PAT environment variable
  # by default; it can also be set explicitly here.
  # personal_access_token = "..."
  timeout_seconds = 30
}
