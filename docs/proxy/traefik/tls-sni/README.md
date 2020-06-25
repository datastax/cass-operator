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

1. Generate the TLS certificates and add them as secrets to the cluster with the guide in the [ssl](../ssl) directory.

1. Install TLS Options to add support for mutual TLS. This configures the CA that must be used in the client certificate

    ```bash
    kubectl apply -f traefik/tls-sni/sample-cluster-sample-dc.tlsoption.yaml
    ```

1. Edit and create the `IngressTCPRoute`. This provides the SNI mapping for routing TCP requests from the ingress to individual pods.

    ```bash
    kubectl apply -f traefik/tls-sni/sample-cluster-sample-dc.ingressroutetcp.yaml
    ```

1. Create the `service` for the pod with `kubectl expose`. Note the service name will match the pod name.

   ```bash
   kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-0
   kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-1
   kubectl expose pod sample-cluster-sample-dc-sample-rack-sts-2
   ```

1. Test connecting with a simple Java application.

   ```java
   package com.datastax.examples;

   import com.datastax.oss.driver.api.core.CqlSession;
   import com.datastax.oss.driver.api.core.cql.ResultSet;
   import com.datastax.oss.driver.internal.core.metadata.SniEndPoint;
   import com.datastax.oss.driver.internal.core.ssl.SniSslEngineFactory;

   import javax.net.ssl.KeyManagerFactory;
   import javax.net.ssl.SSLContext;
   import javax.net.ssl.TrustManagerFactory;
   import java.net.InetSocketAddress;
   import java.security.KeyStore;
   import java.security.SecureRandom;

   public class SampleApp
   {
       public static void main( String[] args ) throws Exception
       {
           SampleApp app = new SampleApp();
           app.run();
       }

       public void run() throws Exception {
           // Configre the SSLEngineFactory
           char[] password = "foobarbaz".toCharArray();

           KeyStore ks = KeyStore.getInstance("JKS");
           ks.load(getClass().getResourceAsStream("/client.keystore"), password);
           KeyManagerFactory kmf = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
           kmf.init(ks, password);

           KeyStore ts = KeyStore.getInstance("JKS");
           ts.load(getClass().getResourceAsStream("/client.truststore"), password);
           TrustManagerFactory tmf =
                   TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
           tmf.init(ts);

           SSLContext sslContext = SSLContext.getInstance("SSL");
           sslContext.init(kmf.getKeyManagers(), tmf.getTrustManagers(), new SecureRandom());

           SniSslEngineFactory sslEngineFactory = new SniSslEngineFactory(sslContext);

           // Proxy address
           InetSocketAddress proxyAddress = new InetSocketAddress("traefik.k3s.local", 9042);

           // Endpoint (contact point)
           SniEndPoint endPoint = new SniEndPoint(proxyAddress, "d1ba31b6-4b0e-4a7a-ba7e-8721271ae99a");

           CqlSession session = CqlSession.builder()
                   .addContactEndPoint(endPoint)
                   .withLocalDatacenter("sample-dc")
                   .withSslEngineFactory(sslEngineFactory)
                   .withCloudProxyAddress(proxyAddress)
                   .build();

           ResultSet rs = session.execute("SELECT data_center, rack, host_id, release_version FROM system.local");
           System.out.println(rs.one().getFormattedContents());

           session.close();
       }
   }
   ```

   If everything worked correctly you should see some information from a node in the cluster.

   ```text
   [data_center:'sample-dc', rack:'sample-rack', host_id:d1ba31b6-4b0e-4a7a-ba7e-8721271ae99a, release_version:'3.11.6']
   ```
