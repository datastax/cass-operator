## Apache Cassandra

The Apache Cassandra database is the right choice when you need scalability and
high availability without compromising performance. Linear scalability and
proven fault-tolerance on commodity hardware or cloud infrastructure make it the
perfect platform for mission-critical data. Cassandra's support for replicating
across multiple datacenters is best-in-class, providing lower latency for your
users and the peace of mind of knowing that you can survive regional outages.

## DataStax Enterprise

The most advanced distribution of Apache Cassandra™ on the market, with the
enterprise functionality needed for serious production systems and backed up and
supported by the best distributed-experts in the world. It's one platform for
all types of applications anywhere, any cloud, any model: key-value, graph,
tabular, JSON.

DataStax Enterprise is a fully integrated and optimized database, with graph,
analytics, and search included, all with a unified security model. Simply put,
it's the only database capable of meeting today's demanding requirements

## Operator Details

`cass-operator` is designed as a modular operator for Apache Cassandra and
derived  distributions. Apache Cassandra is a distributed database consisting of
multiple nodes working in concert to store data and process queries along a
number of fault domains. `cass-operator` has the deployment of a Cassandra
cluster around the logical domain of a datacenter with the `CassandraDatacenter`
custom resource. Upon submission of one of these resources it handles
provisioning the underlying stateful sets (analogous to C\* logical racks),
services, and configuration. Additionally through monitoring pod state via
Kubernetes callbacks it handles day to day operations such as restarting failed
processes, scaling clusters up, and deploying configuration changes in a
rolling, non-disruptive, fashion. This operator is designed to be `Namespace`
scoped. A single Kubernetes cluster may be running multiple instances of this
operator, in separate namespaces, to support a number of C\* clusters and
environments. Configuration is simple with the usage of YAML based overrides in
the Custom Resource paired with an `init` container. In C\* clusters ordering and
timing of certain operations are important to keep the system evenly
distributed. `cass-operator` takes advantage of a sidecar process within the
main container to handle the orchestration of starting our main C* process.

## Pre-requisites

### WebHook Secret

This secret is intentionally left blank and will be filled in as the operator is
brought is started and certificates are generated.

```yaml
apiVersion: v1
data:
  tls.crt: ""
  tls.key: ""
kind: Secret
metadata:
  name: cass-operator-webhook-config
```

### WebHook Configuration

Note the namespace _must_ be updated in the configuration below.

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingWebhookConfiguration
metadata:
  name: "cassandradatacenter-webhook-registration"
webhooks:
- name: "cassandradatacenter-webhook.cassandra.datastax.com"
  rules:
  - apiGroups:   ["cassandra.datastax.com"]
    apiVersions: ["v1beta1"]
    operations:  ["CREATE", "UPDATE", "DELETE"]
    resources:   ["cassandradatacenters"]
    scope:       "*"
  clientConfig:
    service:
      name: "cassandradatacenter-webhook-service"
      namespace: "cass-operator"
      path: /validate-cassandra-datastax-com-v1beta1-cassandradatacenter
  admissionReviewVersions: ["v1beta1"]
  failurePolicy: "Ignore"
  matchPolicy: "Equivalent"
  sideEffects: None
  timeoutSeconds: 10
```

### WebHook Service

Note the namespace _must_ be updated in the configuration below.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cassandradatacenter-webhook-service
  labels:
    name: cass-operator-webhook
spec:
  ports:
  - port: 443
    targetPort: 8443
  selector:
    name: cass-operator
```
