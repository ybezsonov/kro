data "aws_region" "current" {}
data "aws_caller_identity" "current" {}
data "aws_availability_zones" "available" {
  # Do not include local zones
  filter {
    name   = "opt-in-status"
    values = ["opt-in-not-required"]
  }
}
data "aws_ecr_authorization_token" "token" {}

data "aws_iam_session_context" "current" {
  # This data source provides information on the IAM source role of an STS assumed role
  # For non-role ARNs, this data source simply passes the ARN through issuer ARN
  # Ref https://github.com/terraform-aws-modules/terraform-aws-eks/issues/2327#issuecomment-1355581682
  # Ref https://github.com/hashicorp/terraform-provider-aws/issues/28381
  arn = data.aws_caller_identity.current.arn
}

# External data source to read environment variables
data "external" "env_vars" {
  program = ["bash", "-c", "echo '{\"IDE_PASSWORD\":\"'\"$IDE_PASSWORD\"'\", \"GITEA_EXTERNAL_URL\":\"'\"$GITEA_EXTERNAL_URL\"'\", \"GITEA_USERNAME\":\"'\"$GITEA_USERNAME\"'\", \"GITEA_PASSWORD\":\"'\"$GITEA_PASSWORD\"'\"}'"]
}
