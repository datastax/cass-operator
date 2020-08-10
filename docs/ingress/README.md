# Connecting applications to Cassandra running on Kubernetes

## Background

As long as applications run within a Kubernetes (k8s) cluster there will be a need to access those services from outside of the cluster. Connecting to a Cassandra (C*) cluster running within k8s can range from trivial to complex depending on where the client is running, latency requirements, and / or security concerns. This document aims to provide a number of solutions to these issues along with the rationale and motivation for each. The following approaches all assume a C* cluster is already up and reported as running.

## Pod Access

Any pod running within a Kubernetes cluster may communicate with any other pod given the container network policies permit it. Most communication and service discovery within a K8s cluster will not be an issue.

### Network Supported Direct Access

The simplest method, from an architecture perspective, for communicating with Cassandra pods involves having Kubernetes run in an environment where the pod network address space is known and advertised with routes at the network layer. In these types of environments, BGP and static routes may be defined at layer 3 in the OSI model. This allows for IP connectivity / routing directly to pods and services running within Kubernetes from **both** inside and outside the cluster. Additionally, this approach will allow for the consumption of service addresses externally. Unfortunately, this requires an advanced understanding of both k8s networking and the infrastructure available within the enterprise or cloud where it is hosted.

**Pros**

* Zero additional configuration within the application
* Works inside and outside of the Kubernetes network

**Cons**

* Requires configuration at the networking layer within the cloud / enterprise environment
* Not all environments can support this approach. Some cloud environments do not have the tooling exposed for customers to enable this functionality.

### Host Network

Host Network configuration exposes all network interfaces to the underlying pod instead of a single virtual interface. This will allow Cassandra to bind on the worker's interface with an externally accessible IP. Any container that is launched as part of the pod will have access to the host's interface, it cannot be fenced off to a specific container.

Enabling this behavior is done by passing hostNetwork: true in the podTemplateSpec at the top level.

**Pros**

* External connectivity is possible as the service is available at the nodes IP instead of an IP internal to the Kubernetes cluster.

**Cons**

* If a pod is rescheduled the IP address of the pod can change
* In some K8s distributions this is a privileged operation
* Additional automation will be required to identify the appropriate IP and set it for listen_address and broadcast_address
* Only one Cassandra pod may be started per worker, regardless of `allowMultiplePodsPerWorker` setting.

### Host Port

Host port is similar to host network, but instead of being applied at the pod level, it is applied to specified containers within the pod. For each port listed in the container's block a hostPort: external_port key value is included. external_port is the port number on the Kubernetes worker that should be forwarded to this container's port.

At this time we do not allow for modifying the cassandra container via podTemplateSpec, thus configuring this value is not possible without patching each rack's stateful set.

**Pros**

* External connectivity is possible as the service is available at the nodes IP instead of an IP internal to the Kubernetes cluster.
* Easier configuration a separate container to determine the appropriate IP is no longer required.

**Cons**

* If a pod is rescheduled the IP address of the pod can change
* In some K8s distributions this is a privileged operation
* Only one Cassandra pod may be started per worker, regardless of allowMultiplePodsPerWorker setting.
* Not recommended according to K8s [Configuration Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/#services).

## Services

If the application is running within the same Kubernetes cluster as the Cassandra cluster connectivity is simple. cass-operator exposes a number of services representing a Cassandra cluster, datacenters, and seeds. Applications running within the same Kubernetes cluster may leverage these services to discover and identify pods within the target C* cluster.

External applications do not have access to this information via DNS as internal applications do. It is possible to forward DNS requests to Kubernetes from outside the cluster and resolve configured services. Unfortunately, this will provide the internal pod IP addresses and not those routable unless Network Supported Direct Access is possible within the environment. In most scenarios, external applications will not be able to leverage the exposed services from cass-operator.

### Load Balancer

It is possible to configure a service within Kubernetes outside of those provided by cass-operator that is accessible from outside of the Kubernetes cluster. These services have a type: LoadBalancer key in the spec block. In most cloud environments this results in a native cloud load balancer being provisioned to point at the appropriate pods with an external IP. Once the load balancer is provisioned running kubectl get svc will display the external IP address that is pointed at the C* nodes.

**Pros**

* Available from outside of the cluster

**Cons**

* Requires use of an `AddressTranslator` client side to restrict attempts by the drivers to connect directly with pods and instead direct connnections to the load balancer.
* Removes the possibility of TokenAwarePolicy LBP
* Does not support TLS termination at the service layer, but rather within the application.

## Ingresses

Ingresses forward requests to services running within a Kubernetes cluster based on rules. These rules may include specifying the protocol, port, or even path. They may provide additional functionality like termination of SSL / TLS traffic, load balancing across a number of protocols, and name-based virtual hosting. Behind the Ingress K8s type is an Ingress Controller. There are a number of controllers available with varying features to service the defined ingress rules. Think of Ingress as an interface for routing and an Ingress Controller as the implementation of that interface. In this way, any number of Ingress Controllers may be used based on the workload requirements. Ingress Controllers function at Layer 4 & 7 of the OSI model.

When the ingress specification was created it focused specifically on HTTP / HTTPS workloads. From the documentation, "An Ingress does not expose arbitrary ports or protocols. Exposing services other than HTTP and HTTPS to the internet typically uses a service of type Service.`Type=NodePort` or Service.`Type=LoadBalancer`." Cassandra workloads do NOT use HTTP as a protocol, but rather a specific TCP protocol.

Ingress Controllers we are looking to leverage require support for TCP load balancing. This will provide routing semantics similar to those of LoadBalancer Services. If the Ingress Controller also supports SSL termination with [SNI](https://en.wikipedia.org/wiki/Server_Name_Indication). Then secure access is possible from outside the cluster while _keeping Token Aware routing support_. Additionally, operators should consider whether the chosen Ingress Controller supports client SSL certificates allowing for [mutual TLS](https://en.wikipedia.org/wiki/Mutual_authentication) to restrict access from unauthorized clients.

**Pros**

* Highly-available, entrypoint in to the cluster
* _Some_ implementations support TCP load balancing
* _Some_ implementations support Mutual TLS
* _Some_ implementations support SNI

**Cons**

* No _standard_ implementation. Requires careful selection.
* Initially designed for HTTP/HTTPS only workloads
  * Many ingresses support pure TCP workloads, but it is _NOT_ defined in the original design specification. Some configurations require fairly heavy handed templating of base configuration files. This may lead to difficult upgrade paths of those components in the future.
* _Only some_ implementations support TCP load balancing
* _Only some_ implementations support mTLS
* _Only some_ implementations support SNI with TCP workloads

### Kong

[Kong](https://konghq.com/kong/) is open source API gateway. Built for multi-cloud and hybrid, optimized for microservices and distributed architectures. Kong does not have to be deployed on Kubernetes supporting a multitude of environments. For our use case we will install Kong as an ingress for a Kubernetes cluster.

#### Sample Implementations

* [Simple Ingress](kong/ingress)
* [SNI Ingress](kong/sni-ingress)
* [mTLS with SNI Ingress](kong/mtls-sni-ingress)

### Traefik

[Traefik](https://containo.us/traefik/) is an open-source Edge Router that is designed to work in a number of environments, not just Kubernetes. When running on Kubernetes, Traefik is generally installed as an Ingress Controller. Traefik supports TCP load balancing along with SSL termination and SNI.  It is automatically included as the default Ingress Controller of [K3s](https://k3s.io/) and [K3d](https://k3d.io/).

#### Sample Implementations

* [Simple load balancing](traefik/load-balancing)
* [mTLS with load balancing](traefik/mtls-load-balancing)
* [mTLS with SNI](traefik/mtls-sni)

## Service Meshes


## Java Driver Configuration

Each of the three reference implementations has a corresponding configuration in the [sample application](sample-java-application) with associated configuration files and sample code.

## Sample `CassandraDatacenter` Reference

See [`sample-cluster-sample-dc.yaml`](sample-cluster-sample-dc.yaml)

## SSL Certificate Generation

See [ssl/README.md](ssl/README.md) for directions around creating a CA, client, and ingress certificates.

## References

1. [Accessing Kubernetes Pods from Outside of the Cluster](http://alesnosek.com/blog/2017/02/14/accessing-kubernetes-pods-from-outside-of-the-cluster/)
1. [Traefik Docs](https://docs.traefik.io/)
1. [Kubernetes Configuration Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/#services)
