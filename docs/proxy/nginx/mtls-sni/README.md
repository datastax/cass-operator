# NGINX with TLS and SNI

When leveraging a single endpoint ingress / load balancer we naturally remove the ability to route requests based on token awareness. That is unless we leverage TLS with SNI. In this approach the TLS Client HELLO includes a server name which allows the single endpoint to forward the request to the appropriate pod based on rules we specify.

1. _Optional_ provision a local cluster with k3d. If you already have a cluster provisioned and it is available via `kubectl` you may safely skip this step.

    ```bash
    # Create the cluster
    k3d c -x "--no-deploy" -x "traefik"

    # Import images from the local Docker daemon
    k3d i datastax/cass-operator:1.2.0
    k3d i datastax/cassandra:3.11.6-ubi7
    k3d i datastax/cass-config-builder:1.0.0-ubi7
    ```

1. Install the NGINX ingress controller via Helm

    ```bash
    helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
    helm install ingress-nginx ingress-nginx/ingress-nginx
    ```

1. Install `cass-operator` via Helm

    ```bash
    helm install --namespace=default cass-operator ./charts/cass-operator-chart
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f sample-cluster-sample-dc.yaml
    ```

