# Traefik

[Traefik](https://containo.us/traefik/) is an open-source Edge Router that is designed to work in a number of environments, not just Kubernetes. When running on Kubernetes, Traefik is generally installed as an Ingress Controller. Traefik supports TCP load balancing along with SSL termination and SNI.  

1. Install Traefik with Helm

   ```bash
   helm repo add traefik https://containous.github.io/traefik-helm-chart
   helm repo update
   helm install traefik traefik/traefik
   ```

1. Add an ingress route for the Traefik dashboard and get the IP of the load balancer

   ```bash
   kubectl apply -f traefik/tls-sni/dashboard.ingressroute.yaml
   kubectl get svc traefik -o jsonpath="{.status.loadBalancer.ingress[].ip} traefik.k3s.local"
   ```

   If you add an entry to your hosts file with the value from the second command the Traefik dashboard may be viewed at http://traefik.k3s.local/dashboard/.

1. Install `cass-operator` via Helm

    ```bash
    helm install --namespace=default cass-operator ./charts/cass-operator-chart
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f sample-cluster-sample-dc.yaml
    ```

1. Edit the traefik `deployment` and add an entrypoint for TCP Cassandra traffic. This should be done in the `args` section of the `traefik` container.

    ```bash
    kubectl edit deployment traefik
    ```

    ```yaml
        - --entryPoints.websecure.address=:8443/tcp
        # Add the following line, note the port number does have to be 9042. The value "cassandra" is displayed in the Traefik UI.
        - --entryPoints.cassandra.address=:9042/tcp
        - --api.dashboard=true
    ```

    After saving your changes the deployment will replace the old pod with a new one including the adjusted arguments. Validate the new entrypoint exists in the Traefik dashboard.

1. With a new EntryPoint defined we must update the existing service with the new ports.

    ```bash
    kubectl edit svc traefik
    ```

    ```yaml
      - name: websecure
        nodePort: 31036
        port: 443
        protocol: TCP
        targetPort: websecure
      # Add the following section
      - name: cassandra
        port: 9042
        protocol: TCP
        targetPort: 9042
    ```

1. Query the host ID values used in the cluster

    ```bash
    kubectl get cassdc -o json | jq ".items[].status.nodeStatuses"
    {
      "sample-cluster-sample-dc-sample-rack-sts-0": {
        "hostID": "d1ba31b6-4b0e-4a7a-ba7e-8721271ae99a",
        "nodeIP": "10.42.0.29"
      }
    }
    ```

1. Generate the TLS certificates and add them as secrets to the cluster with the guide in the [ssl](ssl) directory.

1. Install TLS Options to add support for mutual TLS. This configures the CA that must be used in the client certificate

    ```bash
    kubectl apply -f traefik/tls-sni/sample-cluster-sample-dc.tlsoption.yaml
    ```

1. Edit and create the `IngressTCPRoute`. This provides the SNI mapping for routing TCP requests from the ingress to individual pods.

    ```bash
    kubectl apply -f traefik/tls-sni/sample-cluster-sample-dc.ingressroutetcp.yaml
    ```

1. Create the `service` for the pod

   ```bash
   kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-0
   ```