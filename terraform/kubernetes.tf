resource "scaleway_k8s_cluster" "demo" {
  name    = "demo"
  version = "1.22.3"
  cni     = "cilium"
}

resource "scaleway_k8s_pool" "demo" {
  cluster_id         = scaleway_k8s_cluster.demo.id
  name               = "demo"
  node_type          = "DEV1-M"
  size               = 3
  min_size           = 3
  max_size           = 3
  autoscaling        = true
  autohealing        = true
}

resource "null_resource" "kubeconfig" {
  depends_on = [scaleway_k8s_pool.demo] # at least one pool here
  triggers = {
    host                   = scaleway_k8s_cluster.demo.kubeconfig[0].host
    token                  = scaleway_k8s_cluster.demo.kubeconfig[0].token
    cluster_ca_certificate = scaleway_k8s_cluster.demo.kubeconfig[0].cluster_ca_certificate
  }
}

resource "kubernetes_job" "vault-config" {
  depends_on = [helm_release.vault]
  metadata {
    name = "vault-config"
    namespace = "default"
  }
  spec {
    template {
      metadata {}
      spec {
        service_account_name = "vault-config"
        container {
          name    = "vault-config"
          image   = "golgoth31/vault-config:latest"
        }
        restart_policy = "Never"
      }
    }
    backoff_limit = 0
  }
  wait_for_completion = false
}

resource "kubernetes_service_account" "vault-config" {
  depends_on = [helm_release.vault]
  metadata {
    name = "vault-config"
    namespace = "default"
  }
}

resource "kubernetes_role" "vault-config" {
  depends_on = [helm_release.vault]
  metadata {
    name = "vault-config"
    namespace = "default"
  }

  rule {
    api_groups     = [""]
    resources      = ["secrets"]
    verbs          = ["get", "list", "watch", "create", "update", "patch", "delete"]
  }
}

resource "kubernetes_role_binding" "vault-config" {
  depends_on = [helm_release.vault]
  metadata {
    name = "vault-config"
    namespace = "default"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = "vault-config"
  }
  subject {
    kind      = "ServiceAccount"
    name      = "vault-config"
    namespace = "default"
  }
}

resource "kubernetes_secret" "vault-kms" {
  depends_on = [helm_release.vault]
  metadata {
    name = "vault-kms"
  }

  data = {
    VAULT_SEAL_TYPE="awskms"
    VAULT_AWSKMS_SEAL_KEY_ID="${aws_kms_key.kms.key_id}"
    AWS_DEFAULT_REGION="eu-west-3"
    AWS_ACCESS_KEY_ID="${aws_iam_access_key.kms.id}"
    AWS_SECRET_ACCESS_KEY="${aws_iam_access_key.kms.secret}"
  }
}

resource "local_file" "kubeconfig" {
  depends_on   = [scaleway_k8s_cluster.demo]
  filename     = "./kubeconfig"
  content      = scaleway_k8s_cluster.demo.kubeconfig[0].config_file
}
