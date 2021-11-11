output "instance_ips" {
  value = [scaleway_k8s_pool.demo.nodes[*].public_ip]
}

output "kubeconfig" {
  value = scaleway_k8s_cluster.demo.kubeconfig[0].config_file
}
