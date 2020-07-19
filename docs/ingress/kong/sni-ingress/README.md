# Kong SNI Ingress

When leveraging a single endpoint ingress / load balancer we lose the ability to provide token aware routing without the use of SNI. SNI hints to the ingress (via TLS extensions) where the traffic should be routed from the proxy. In this case we use the hostId as the endpoint.

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
      datastax/dse-server:6.8.0-ubi7 \
      datastax/cassandra:3.11.6-ubi7
    ```

1. Install `cass-operator` via Helm

    ```bash
    helm repo add datastax https://datastax.github.io/charts
    helm repo update
    helm install cass-operator datastax/cass-operator
    ```

1. Deploy a Cassandra cluster

    ```bash
    kubectl apply -f docs/ingress/sample-cluster-sample-dc.yaml
    ```

1. Expose each pod as a service, **AFTER all pods are up and read**

    ```bash
    kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-0
    kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-1
    kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-2
    ```

1. Generate and install [SSL certificates](../../ssl)

1. Install Kong with Helm

    ```bash
    helm repo add kong https://charts.konghq.com
    helm repo update
    helm install kong kong/kong \
      --set ingressController.installCRDs=false \
      --set admin.enabled=true \
      --set admin.http.enabled=true \
      --set admin.servicePort=8001 \
      --set admin.type=LoadBalancer
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
                  # Note the port must match the `port` value in the patched service
                  value: 0.0.0.0:9042 ssl
              ports:
                - name: cassandra
                  # Note this must match the `port` value in the patched service
                  containerPort: 9042
                  protocol: TCP'
    ```

1. Update the Kong service to include the port we want to forward from.

    ```bash
    kubectl patch svc kong-kong-proxy --patch '
    spec:
      ports:
        # Note the `port` field can be any value. When running multiple clusters they must be different. `targetPort` *must* match the port C* is listening on, default: 9042
        - name: cassandra
          port: 9042
          protocol: TCP
          targetPort: 9042'
    ```

1. Create a `TCPIngress`. This provides the mapping between Kong ingress and the internal Cassandra service.

    ```bash
    kubectl apply -f docs/ingress/kong/sni-ingress/sample-cluster-sample-dc.tcpingress.yaml
    ```

1. Check out the [sample application](../../sample-java-application) to validate your deployment
    
    ```bash
    Discovered Nodes
    sample-dc:sample-rack:bbbf5a34-2240-4efb-ac06-c7974a2ec3dd
    sample-dc:sample-rack:73a03b32-bdb6-4b2a-a5db-dfd078ec8131
    sample-dc:sample-rack:deab7ace-711c-407f-96a0-bcba5099855b

    Coordinator: sample-dc:sample-rack:73a03b32-bdb6-4b2a-a5db-dfd078ec8131
    [data_center:'sample-dc', rack:'sample-rack', host_id:73a03b32-bdb6-4b2a-a5db-dfd078ec8131, release_version:'3.11.6']

    Coordinator: sample-dc:sample-rack:73a03b32-bdb6-4b2a-a5db-dfd078ec8131
    [data_center:'sample-dc', rack:'sample-rack', host_id:bbbf5a34-2240-4efb-ac06-c7974a2ec3dd, release_version:'3.11.6']
    [data_center:'sample-dc', rack:'sample-rack', host_id:deab7ace-711c-407f-96a0-bcba5099855b, release_version:'3.11.6']
    ```
