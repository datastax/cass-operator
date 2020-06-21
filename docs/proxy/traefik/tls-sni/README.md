# Traefik

[Traefik](https://containo.us/traefik/) is an open-source Edge Router that is designed to work in a number of environments, not just Kubernetes. When running on Kubernetes, Traefik is generally installed as an Ingress Controller. Traefik supports TCP load balancing along with SSL termination and SNI.  It is automatically included as the default Ingress Controller of [K3s](https://k3s.io/) and [K3d](https://k3d.io/).

1. _Optional_ - if Traefik is not already installed, then follow this step

   ```bash
   helm repo add traefik https://containous.github.io/traefik-helm-chart
   helm repo update
   helm install traefik traefik/traefik
   ```

1. _Optional_ - if Traefik was preinstalled ensure that the ClusterRoleBinding is updated and CustomResourceDefinitions are installed

    ```bash
    kubectl apply -f traefik/clusterrole.yaml
    kubectl apply -f traefik/customresourcedefinition.yaml
    ```

1. Install `cass-operator` via Helm

    ```bash
    helm install --namespace=default cass-operator ./charts/cass-operator-chart
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f sample-cluster-sample-dc.yaml
    ```

1. Edit the traefik `configmap` and include the `entryPoints.cassandra-tls` block found in [traefik/configmap.yaml](traefik/configmap.yaml). Note that any port value and name may be used here, a single ingress may service multiple clusters.

    ```bash
    kubectl edit configmap traefik -n kube-system
    ```

    Note our configmap also includes an `api` block.

1. With the config map updated the fastest way to see those changes reflected is to delete the existing pod. The deployment will handle recreating it with the updated config.

    ```bash
    kubectl delete pod -n kube-system -l app=traefik
    ```

1. With a new EntryPoint defined we must update the existing service with the new ports.

    ```bash
    kubectl edit svc traefik -n kube-system
    ```

1. Query the host ID values used in the cluster

    ```bash
    kubectl get cassdc -o json | jq ".items[0].status.nodeStatuses"
    {
      "sample-cluster-sample-dc-sample-rack-sts-0": {
        "hostID": "b8f2960c-0192-45ce-9c90-9ad57ba9c19e",
        "nodeIP": "10.42.0.29"
      }
    }
    ```

1. Generate the TLS certificates and add them as secrets to the cluster with the guide in the [ssl](ssl) directory.

1. Install TLS Options to add support for mutual TLS

    ```bash
    kubectl apply -f traefik/sample-cluster-sample-dc.tlsoption.yaml
    ```

1. Edit and create the `IngressTCPRoute`. This provides the SNI mapping for routing TCP requests from the ingress to individual pods.

    ```bash
    kubectl apply -f traefik/sample-cluster-sample-dc.ingressroutetcp.yaml
    ```
