# Kong Simple Load Balancing

When leveraging a single endpoint ingress / load balancer we lose the ability to provide token aware routing without the use of SNI (see the [mTLS with SNI guide](../mtls-sni)). **WARNING** This approach does not interact with the traffic at all. All traffic is sent over cleartext without any form of authentication of the server or client.. Note that **_each_** Cassandra cluster running behind the ingress will require it's own endpoint / port. Without a way to detect the pod we want to connect with it's the only way to differentiate requests.

1. _Optional_ provision a local cluster with k3d. If you already have a cluster provisioned and it is available via `kubectl` you may safely skip this step.

   ```bash
   # Create the cluster
   k3d create cluster --k3s-server-arg --no-deploy --k3s-server-arg traefik
   export KUBECONFIG="$(k3d get-kubeconfig --name='k3s-default')"
   kubectl cluster-info

   # Import images from the local Docker daemon
   k3d load image --cluster k3s-default \
     datastax/cass-operator:1.3.0 \
     datastax/cass-config-builder:1.0.0-ubi7 \
     datastax/dse-server:6.8.0-ubi7
   ```

1. Install Kong with Helm

   ```bash
   helm repo add kong https://charts.konghq.com
   helm repo update
   helm install kong kong/kong --set ingressController.installCRDs=false
   ```

1. Update the Kong service to include the port we want to forward from.

    ```bash
    kubectl edit svc kong-kong-proxy
    ```

    ```yaml
      - name: kong-proxy
        nodePort: 30374
        port: 80
        protocol: TCP
        targetPort: 8000
      - name: kong-proxy-tls
        nodePort: 32570
        port: 443
        protocol: TCP
        targetPort: 8443
      # Add the following section, it is ideal to use the same name as your entrypoint. Additionally the port number MUST match
      - name: cassandra
        port: 9042
        protocol: TCP
        targetPort: 9042
    ```

1. Install `cass-operator` via Helm

    ```bash
    helm install cass-operator ./charts/cass-operator-chart
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f sample-cluster-sample-dc.yaml
    ```

1. Patch the Kong deployment to listen on the ingress port (9042 in our example)
   
    ```bash
    kubectl patch deployment kong-kong --patch '
    spec:
      template:
        spec:
          containers:
            - name: proxy
              env:
                - name: KONG_STREAM_LISTEN
                  value: 0.0.0.0:9042
              ports:
                - name: cassandra
                  containerPort: 9042
                  protocol: TCP
    '
    ```

1. Create a `TCPIngress`. This provides the mapping between Kong ingress and the internal Cassandra service.

    ```bash
    kubectl apply -f kong/load-balancing/sample-cluster-sample-dc.tcpingress.yaml
    ```

1. Check out the [sample application](../../sample-java-application) to validate your deployment
