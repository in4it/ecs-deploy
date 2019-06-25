# SAML

# Instructions:
# Create new key using: openssl req -x509 -newkey rsa:2048 -keyout myservice.key -out myservice.cert -days 3650 -nodes -subj "/CN=myservice.mycompany.com"
# Create SAML_CERTIFICATE and SAML_PRIVATE_KEY in SSM parameter store

resource "aws_ssm_parameter" "ecs-deploy-saml-enabled" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/SAML_ENABLED"
  type  = "String"
  value = "yes"
  count = var.saml_enabled == "yes" ? 1 : 0
}

resource "aws_ssm_parameter" "ecs-deploy-saml-acs-url" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/SAML_ACS_URL"
  type  = "String"
  value = var.saml_acs_url == "" ? "https://${var.cluster_domain}${var.url_prefix}" : var.saml_acs_url
  count = var.saml_enabled == "yes" ? 1 : 0
}

resource "aws_ssm_parameter" "ecs-deploy-saml-metadata-url" {
  name  = "/${var.cluster_name}-${var.aws_env}/ecs-deploy/SAML_METADATA_URL"
  type  = "String"
  value = var.saml_metadata_url
  count = var.saml_enabled == "yes" ? 1 : 0
}

