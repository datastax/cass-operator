# Traefik Simple Load Balancing with mTLS

When leveraging a single endpoint ingress / load balancer we lose the ability to provide token aware routing without the use of SNI (see the [mTLS with SNI guide](../mtls-sni)). This approach keeps unwanted traffic from reaching the cluster through the use of mTLS terminated at the ingress layer. Note that **_each_** Cassandra cluster running behind the ingress will require it's own endpoint. Without a way to detect the pod we want to connect with it's the only way to differentiate requests.

1. _Optional_ provision a local cluster with k3d. If you already have a cluster provisioned and it is available via `kubectl` you may safely skip this step.

   ```bash
   # Create the cluster
   k3d c -x "--no-deploy" -x "traefik"
   export KUBECONFIG="$(k3d get-kubeconfig --name='k3s-default')"
   kubectl cluster-info

   # Import images from the local Docker daemon
   k3d i datastax/cass-operator:1.2.0
   k3d i datastax/cassandra:3.11.6-ubi7
   k3d i datastax/cass-config-builder:1.0.0-ubi7
   ```

1. Install Traefik with Helm

   ```bash
   helm repo add traefik https://containous.github.io/traefik-helm-chart
   helm repo update
   helm install traefik traefik/traefik
   ```

1. Add an ingress route for the Traefik dashboard and get the IP of the load balancer

   ```bash
   kubectl apply -f traefik/dashboard.ingressroute.yaml
   kubectl get svc traefik -o jsonpath="{.status.loadBalancer.ingress[].ip} traefik.k3s.local"
   ```

   If you add an entry to your /etc/hosts file with the value from the second command. With this in place the Traefik dashboard may be viewed at http://traefik.k3s.local/dashboard/.

1. Edit the traefik `deployment` and add an entrypoint for TCP Cassandra traffic. This should be done in the `args` section of the `traefik` container.

    ```bash
    kubectl edit deployment traefik
    ```

    ```yaml
        - --entryPoints.websecure.address=:8443/tcp
        # Add the following line, note the port number does have to be 9042. The value "cassandra" is displayed in the Traefik UI and may also be customized
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
      # Add the following section, it is ideal to use the same name as your entrypoint. Additionally the port number MUST match
      - name: cassandra
        port: 9042
        protocol: TCP
        targetPort: 9042
    ```

    At this point refreshing the Traefik dashboard should show a new endpoint named `cassandra` running.

1. Install `cass-operator` via Helm

    ```bash
    helm install --namespace=default cass-operator ./charts/cass-operator-chart
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f sample-cluster-sample-dc.yaml
    ```

1. Generate the TLS certificates and add them as secrets to the cluster with the guide in the [ssl](../ssl) directory. Note you do **NOT** need to specify any of the host ID values as we will not be performing additional routing at the ingress layer.

1. Install TLS Options to add support for mutual TLS. This configures the CA that must be used in the client certificate

    ```bash
    kubectl apply -f traefik/mtls-load-balancing/sample-cluster-sample-dc.tlsoption.yaml
    ```

1. Create the `IngressTCPRoute`. This provides the mapping between our endpoint and internal service and binds the previously installed tlsoptions to the endpoint.

    ```bash
    kubectl apply -f traefik/mtls-load-balancing/sample-cluster-sample-dc.ingressroutetcp.yaml
    ```

1. Check out the [sample application](../../sample-java-application) to validate your deployment
