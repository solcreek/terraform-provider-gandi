# DNSSEC keys are imported as <domain>/<key_id>. List key ids with the
# gandi_dnssec_keys data source.
terraform import gandi_dnssec_key.example example.com/12345
