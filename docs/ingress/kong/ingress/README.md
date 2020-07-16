# Kong Simple Load Balancing

When leveraging a single endpoint ingress / load balancer we lose the ability to provide token aware routing without the use of SNI (see the [mTLS with SNI guide](../mtls-sni)). **WARNING** This approach does not interact with the traffic at all. All traffic is sent over cleartext without any form of authentication of the server or client.. Note that **_each_** Cassandra cluster running behind the ingress will require it's own endpoint / port. Without a way to detect the pod we want to connect with it's the only way to differentiate requests.

1. _Optional_ provision a local cluster with k3d. If you already have a cluster provisioned and it is available via `kubectl` you may safely skip this step.

   ```bash
   # Create the cluster
   k3d create cluster --k3s-server-arg --no-deploy --k3s-server-arg traefik
   export KUBECONFIG="$(k3d get-kubeconfig --name='k3s-default')"
   kubectl cluster-info
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

1. Install Kong with Helm

   ```bash
   helm repo add kong https://charts.konghq.com
   helm repo update
   helm install kong kong/kong --set ingressController.installCRDs=false
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
                  value: 0.0.0.0:9042
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
    kubectl apply -f docs/ingress/kong/ingress/sample-cluster-sample-dc.tcpingress.yaml
    ```

1. Check out the [sample application](../../sample-java-application) to validate your deployment
    
    ```bash
    mvn exec:exec@ingress
    Discovered Nodes
    sample-dc:sample-rack:270acac9-e7d3-422c-b63f-fc210ce53250
    sample-dc:sample-rack:270acac9-e7d3-422c-b63f-fc210ce53250
    sample-dc:sample-rack:270acac9-e7d3-422c-b63f-fc210ce53250

    Coordinator: sample-dc:sample-rack:270acac9-e7d3-422c-b63f-fc210ce53250
    [data_center:'sample-dc', rack:'sample-rack', host_id:ac8cb07b-80eb-4882-b49d-183e28076840, release_version:'3.11.6']

    Coordinator: sample-dc:sample-rack:270acac9-e7d3-422c-b63f-fc210ce53250
    [data_center:'sample-dc', rack:'sample-rack', host_id:270acac9-e7d3-422c-b63f-fc210ce53250, release_version:'3.11.6']
    [data_center:'sample-dc', rack:'sample-rack', host_id:71683027-8b66-420c-aa87-f16ef48e7846, release_version:'3.11.6']
    ```
