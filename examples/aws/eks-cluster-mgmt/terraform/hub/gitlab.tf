################################################################################
# GitLab Helm Chart
################################################################################

locals {
  # Create the values content using templatefile
  gitlab_values = templatefile("${path.module}/gitlab-initial-values.yaml", {
    DOMAIN_NAME = local.cloudfront_domain_name
    INITIAL_ROOT_PASSWORD = data.external.env_vars.result["IDE_PASSWORD"]
  })
}

resource "helm_release" "gitlab" {
  depends_on = [
    local.cluster_info
  ]

  name       = "gitlab"
  chart      = "${path.module}/../../charts/gitlab"
  timeout    = 600
  values     = [local.gitlab_values]
  create_namespace = false
}
