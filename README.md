# Vault-demo

## Pre-requisites

1. AWS token with ability to create iam user and kms key

1. Scaleway token to create kubernetes cluster

1. OVH token to make acme certificate working

## How to launch

1. Change token into env.sample and source it

1. Change domain name into terraform/data/vault/values.yaml; replace "notrenet.ovh" by your domain name

1. Go to the terraform directory and apply it

    ```bash
    cd terraform
    terrafom init
    terraform plan
    terraform apply
    ```

1. Update the traefik-ovh.yaml.sample secret manifest with your OVH (the only one supported) and apply this secret

1. Get the lb ip from scaleway

    ```bash
    scw lb lb get $(scw lb lb list -o json | jq '.[0].id' | tr -d '"') -o json | jq '.ip[0].ip_address' | tr -d '"'
    ```

1. Add the ip to your /etc/hosts

    ```bash
    <lb ip> vault.<your domain name>
    ```

1. Use the vault cli or your browser to connect to ths vault cluster

    User: demo
    Password: demo

1. The root token can be retreived in a secret "vault-config-init"
