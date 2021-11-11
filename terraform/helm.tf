resource "time_sleep" "wait_60_seconds" {
  depends_on = [scaleway_k8s_cluster.demo]

  create_duration = "120s"
}

resource "helm_release" "vault" {
  depends_on = [time_sleep.wait_60_seconds]
  name       = "vault"
  repository = "https://helm.releases.hashicorp.com"
  chart      = "vault"
  version    = "0.17.1"
  values = [
    "${file("data/vault/values.yaml")}"
  ]
}

resource "helm_release" "traefik" {
  depends_on = [time_sleep.wait_60_seconds]
  name       = "traefik"
  repository = "https://helm.traefik.io/traefik"
  chart      = "traefik"
  version    = "10.6.0"
  values = [
    "${file("data/traefik/values.yaml")}"
  ]
}
