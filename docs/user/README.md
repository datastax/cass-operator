# Introduction

Cass Operator simplifies the process of deploying
and managing Cassandra or DSE in a Kubernetes cluster.

# Install the operator

## Prerequisites

1. A Kubernetes cluster. Kubernetes v1.18 is recommended, but Kubernetes
   v1.15 has been tested.
2. The ability to download images from Docker Hub from within the Kubernetes
   cluster.

## Create a namespace

cass-operator is built to watch over pods running Casandra or DSE in a Kubernetes
namespace. Create a namespace for the cluster with:

```shell
$ kubectl create ns cass-operator
```

For the rest of this guide, we will be using the namespace `cass-operator`. Adjust
further commands as necessary to match the namespace you defined.

## Define a storage class

Kubernetes uses the `StorageClass` resource as an abstraction layer between pods
needing persistent storage and the storage resources that a specific
Kubernetes cluster can provide. We recommend using the fastest type of
networked storage available. On Google Kubernetes Engine, the following
example would define persistent network SSD-backed volumes.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: server-storage
provisioner: kubernetes.io/gce-pd
parameters:
  type: pd-ssd
  replication-type: none
volumeBindingMode: WaitForFirstConsumer
```

The above example can be customized to suit your environment and saved as
`server-storage.yaml`. For the rest of this guide, we'll assume you've
defined a `StorageClass` and named it `server-storage`. You can apply that file and
get the resulting storage classes from Kubernetes with:

```shell
$ kubectl -n cass-operator apply -f ./server-storage.yaml

$ kubectl -n cass-operator get storageclass
NAME                 PROVISIONER            AGE
server-storage       kubernetes.io/gce-pd   1m
standard (default)   kubernetes.io/gce-pd   16m
```

## Deploy the operator

Within this guide, we have joined together a few Kubernetes resources into a
single YAML file needed to deploy the operator. This file defines the
following:

1. `ServiceAccount`, `Role`, and `RoleBinding` to describe a user and set of
   permissions necessary to run the operator. _In demo environments that don't
   have role-based access-control enabled, these extra steps are unnecessary but
   are harmless._
2. `CustomResourceDefinition` for the `CassandraDatacenter` resources used to
   configure clusters managed by the `cass-operator`.
3. Deployment to start the operator in a state where it waits and watches for
   CassandraDatacenter resources.

Generally, `cluster-admin` privileges are required to register a
`CustomResourceDefinition` (CRD). All privileges needed by the operator are
present within the
[operator-manifests YAML](/docs/user).
_Note the operator does not require `cluster-admin` privileges, only the user
defining the CRD requires those permissions._

Apply the manifest above, and wait for the deployment to become ready. You can
watch the progress by getting the list of pods for the namespace, as
demonstrated below:

```shell
$ kubectl -n cass-operator apply -f ./cass-operator-manifests.yaml

$ kubectl -n cass-operator get pod
NAME                               READY   STATUS    RESTARTS   AGE
cass-operator-f74447c57-kdf2p       1/1     Running   0          1h
```

When the pod status is `Running`, the operator is ready to use.

# Provision a Cassandra cluster

The previous section created a new resource type in your Kubernetes cluster, the
`CassandraDatacenter`. By adding `CassandraDatacenter` resources to your namespace, you can
define a cluster topology for the operator to create and monitor. In this
guide, a three node cluster is provisioned, with one datacenter made up of three
racks, with one node per rack.

## Example Config

The following example illustrates a `CassandraDatacenter` resource.

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 3.11.7
  managementApiAuth:
    insecure: {}
  size: 3
  racks:
  - name: rack1
  - name: rack2
  - name: rack3
  resources:
    requests:
      memory: 4Gi
      cpu: 1000m
  storageConfig:
    cassandraDataVolumeClaimSpec:
      storageClassName: server-storage
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
  config:
    cassandra-yaml:
      num_tokens: 8
      authenticator: org.apache.cassandra.auth.PasswordAuthenticator
      authorizer: org.apache.cassandra.auth.CassandraAuthorizer
      role_manager: org.apache.cassandra.auth.CassandraRoleManager
    jvm-options:
      initial_heap_size: 2G
      max_heap_size: 2G
      additional-jvm-opts:
      - -Dcassandra.system_distributed_replication_dc_names=dc1
      - -Dcassandra.system_distributed_replication_per_dc=3
```

Consider customizing the example above to suit your requirements, and save it as
`cluster1-dc1.yaml`. Apply this file via `kubectl` and watch the list of pods as
the operator deploys them. Completing a deployment may take several minutes per
node. The best way to track the operator's progress is by using
`kubectl -n cass-operator describe cassdc dc1` and checking the `status` and events.

```shell
$ kubectl -n cass-operator apply -f ./cluster1-dc1.yaml

$ kubectl -n cass-operator get pods
NAME                            READY   STATUS    RESTARTS   AGE
cass-operator-f74447c57-kdf2p   1/1     Running   0          13m
gke-cluster1-dc1-r1-sts-0       1/1     Running   0          5m38s
gke-cluster1-dc1-r2-sts-0       1/1     Running   0          42s
gke-cluster1-dc1-r3-sts-0       1/1     Running   0          6m7s

$ kubectl -n cass-operator describe cassdc dc1
...
Status:
  Cassandra Operator Progress:  Updating
  Last Server Node Started:     2020-03-10T11:37:28Z
  Super User Upserted:          2020-03-10T11:38:37Z
Events:
  Type     Reason           Age                  From                Message
  ----     ------           ----                 ----                -------
  Normal   CreatedResource  9m49s                cassandra-operator  Created service cluster1-dc1-service
  Normal   CreatedResource  9m49s                cassandra-operator  Created service cluster1-seed-service
  Normal   CreatedResource  9m49s                cassandra-operator  Created service cluster1-dc1-all-pods-service
  Normal   CreatedResource  9m49s                cassandra-operator  Created statefulset cluster1-dc1-r1-sts
  Normal   CreatedResource  9m49s                cassandra-operator  Created statefulset cluster1-dc1-r2-sts
  Normal   CreatedResource  9m49s                cassandra-operator  Created statefulset cluster1-dc1-r3-sts
```

## Cluster and Datacenter

A logical datacenter is the primary resource managed by the
cass-operator. Within a single Kubernetes namespace:

- A single `CassandraDatacenter` resource defines a single-datacenter cluster.
- Two or more `CassandraDatacenter` resources with different `clusterName`'s define
  separate and unrelated single-datacenter clusters. Note the operator
  manages both clusters since they reside within the same Kubernetes namespace.
- Two or more `CassandraDatacenter` resources that have the same `clusterName`
  define a multi-datacenter cluster. The operator will join the
  instances in each datacenter into a logical topology that acts as a single
  cluster.

For this guide, we define a single-datacenter cluster. The cluster is named
`cluster1` with the datacenter named `dc1`.

## Racks

Cassandra defines nodes in a logical topology of datacenters and racks. Much like physical server racks in a datacenter, racks in Cassandra define fault domains. Cassandra will place replicas on separate racks to handle a scenario where an entire rack goes offline. Should this occur multiple replicas remain available. In cloud deployments racks align with availability zones. In this guide we will use `r1`, `r2`, and `r3`.

Prior to version 1.6.0 cass-operator used the `zone` parameter of a `rack` when assigning pods to k8s workers. K8s nodes with a matching `failure-domain.beta.kubernetes.io/zone` label were used during scheduling. With cass-operator 1.6.0 the `zone` field has been _deprecated_. In its place are the `nodeAffinityLabels` fields at both `spec` (datacenter) and `rack` levels. `nodeAffinityLabels` are represented as a simple map of label `key/value` pairs. These labels are matched to labels on K8s workers during scheduling. Consider this example which assumes there are two labels on each node with the keys `dc` and `rack`.

```yaml
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 3.11.7
  managementApiAuth:
    insecure: {}
  size: 3
  nodeAffinityLabels:
    dc: us-va
  racks:
  - name: r1
    nodeAffinityLabels:
      rack: us-va-1
  - name: r2
    nodeAffinityLabels:
      rack: us-va-2
  - name: r3
    nodeAffinityLabels:
      rack: us-va-3
```

Here each rack will receive a single pod. The pod will have its [`spec.affinity.nodeAffinity`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#nodeaffinity-v1-core) configured with the values from `spec.nodeAffinityLabels` _and_ `spec.racks[].nodeAffinityLabels` (note that neither field is required).

If you do not know which labels are available on your nodes consult `kubectl get nodes` and `kubectl get nodes node_name -o jsonpath={.metadata.labels}`.

```console
$ kubectl get nodes
NAME                                       STATUS   ROLES    AGE   VERSION
gke-cluster-1-default-pool-3c7ae875-j7bk   Ready    <none>   33s   v1.18.12-gke.1210
gke-cluster-1-default-pool-3c7ae875-zxz2   Ready    <none>   33s   v1.18.12-gke.1210
gke-cluster-1-default-pool-579202fb-99h2   Ready    <none>   33s   v1.18.12-gke.1210
gke-cluster-1-default-pool-579202fb-vq3q   Ready    <none>   33s   v1.18.12-gke.1210
gke-cluster-1-default-pool-75e14bf9-1bsf   Ready    <none>   32s   v1.18.12-gke.1210
gke-cluster-1-default-pool-75e14bf9-kv9r   Ready    <none>   30s   v1.18.12-gke.1210

$ kubectl get nodes gke-cluster-1-default-pool-3c7ae875-j7bk -o jsonpath={.metadata.labels} | jq
{
  "beta.kubernetes.io/arch": "amd64",
  "beta.kubernetes.io/instance-type": "e2-medium",
  "beta.kubernetes.io/os": "linux",
  "cloud.google.com/gke-boot-disk": "pd-standard",
  "cloud.google.com/gke-nodepool": "default-pool",
  "cloud.google.com/gke-os-distribution": "cos",
  "cloud.google.com/machine-family": "e2",
  "failure-domain.beta.kubernetes.io/region": "us-central1",
  "failure-domain.beta.kubernetes.io/zone": "us-central1-a",
  "kubernetes.io/arch": "amd64",
  "kubernetes.io/hostname": "gke-cluster-1-default-pool-3c7ae875-j7bk",
  "kubernetes.io/os": "linux",
  "node.kubernetes.io/instance-type": "e2-medium",
  "topology.gke.io/zone": "us-central1-a",
  "topology.kubernetes.io/region": "us-central1",
  "topology.kubernetes.io/zone": "us-central1-a"
}
```

To target this particular node with C* pods consider using the `topology.kubernetes.io/region` label at the datacenter level and `topology.kubernetes.io/zone` for a single rack.

_Note you are not limited to a single key/value pair for either field._

## Node Count

The `size` parameter is the number of nodes to run in the datacenter.
For optimal performance, it's recommended to run only one server instance per
Kubernetes worker node. The operator will enforce that limit, and
pods may get stuck in the `Pending` status if there are insufficient Kubernetes
workers available.

We'll assume you have at least three worker nodes available. If you're working
locally with minikube or another setup with a single Kubernetes worker node, you must
reduce the `size` value accordingly, or set the `allowMultipleNodesPerWorker`
parameter to `true`.

## The server image user

If the server image runs as the "cassandra" or "dse" user, then a PodSecurityContext for that user will be defined by cass-operator. Otherwise the server image is assumed to be running as the "root" user and a PodSecurityContext is not defined.

For serverType="dse", the server images run as the "dse" user.

For serverType="cassandra", the cass-operator follows these steps in order to determine which user the docker image is run as:

1. If the CassandraDatacenter.Spec.DockerImageRunsAsCassandra field is set, then that "true" or "false" value will be used.
2. If the serverVersion field is set to "3.11.6", "3.11.7", or "4.0.0", cass-operator assumes the image runs as the "root" user.
3. Otherwise, cass-operator assumes that the server is running as the "cassandra" user.

## Storage

Define the storage with a combination of the previously provisioned storage
class and size parameters. These inform the storage provisioner how much room to
require from the backend.

## Configuring the Database

The `config` key in the `CassandraDatacenter` resource contains the parameters used to
configure the server process running in each pod. In general, it's not necessary to
specify anything here at all. Settings that are omitted from the `config` key will
receive reasonable default values and its quite common to run demo clusters with
no custom configuration.

If you're familiar with configuring Apache Cassandra outside of containers on traditional
operating systems, you may recognize that some familiar configuration parameters
have been specified elsewhere in the `CassandraDatacenter` resource, outside of the
`config` section. These parameters should not be repeated inside of the config
section, the operator will populate them from the `CassandraDatacenter` resource.

For example:
* `cluster_name`, which is normally specified in `cassandra.yaml`
* The rack and datacenter properties

Similarly, the operator will automatically populate any values which must
normally be customized on a per-instance basis. Do not specify these in the
`CassandraDatacenter` resource.

For example:
* `initial_token`
* `listen_address` and other ip-addresses.

A large number of keys and values can be specified in the `config` section, but
the details are currently not well documented. The `config` key data structure
resembles the API for DataStax OpsCenter Lifecycle Manager (LCM) Configuration
Profiles. Translating LCM config profile API payloads to this format is
straightforward. Documentation of this section will be present in future
releases.

## Superuser credentials

By default, a cassandra superuser gets created by the operator. A Kubernetes secret
will be created for it, named `<cluserName>-superuser`. It will contain `username`
and `password` keys.

```shell
# Run these commands AFTER you've created your CassandraDatacenter

$ kubectl -n cass-operator get secret cluster1-superuser
NAME                       TYPE                                  DATA   AGE
cluster1-superuser         Opaque                                2      13m

$ kubectl -n cass-operator get secret cluster1-superuser -o yaml
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: cluster1-superuser
data:
  password: d0g0UXRaTTg0VzVXbENCZVo4WmNqRWVFMGx0SXVvWnhMU0k5allsampBYnVLWU9WRTU2NENSWEpwY2twYjArSDlmSnZOcHdrSExZVU8rTk11N1BJRWhhZkpXM1U0WitsdlI1U3owcUhzWmNjRHQ0enhTSFpzeHRNcEFiMzNXVWQ3R25IdUE=
  username: Y2x1c3RlcjEtc3VwZXJ1c2Vy

$ echo Y2x1c3RlcjEtc3VwZXJ1c2Vy | base64 -D
cluster1-superuser

$ echo 'd0g0UXRaTTg0VzVXbENCZVo4WmNqRWVFMGx0SXVvWnhMU0k5allsampBYnVLWU9WRTU2NENSWEpwY2twYjArSDlmSnZOcHdrSExZVU8rTk11N1BJRWhhZkpXM1U0WitsdlI1U3owcUhzWmNjRHQ0enhTSFpzeHRNcEFiMzNXVWQ3R25IdUE=' | base64 -D
wH4QtZM84W5WlCBeZ8ZcjEeE0ltIuoZxLSI9jYljjAbuKYOVE564CRXJpckpb0+H9fJvNpwkHLYUO+NMu7PIEhafJW3U4Z+lvR5Sz0qHsZccDt4zxSHZsxtMpAb33WUd7GnHuA
```

To instead create a superuser
with your own credentials, you can create a secret with kubectl.

### Example superuser secret creation

```
kubectl create secret generic superuser-secret -f my-secret.yaml
```

To use this new superuser secret, specify the name of the secret from
within the `CassandraDatacenter` config yaml that you load into the cluster:

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  superuserSecretName: superuser-secret
```

## Specifying version and image

With the release of the operator v0.4.0 comes a new way to specify
which version of Cassandra or  DSE and image you want to use. From within the config yaml
for your `CassandraDatacenter` resource, you can use the `serverType`, `serverVersion`, and `serverImage`
spec properties.

`serverType` is required and must be either `dse` or `cassandra`. `serverVersion` is also required,
and the supported versions for DSE are `6.8.0` through `6.8.4`, and for Cassandra it is `3.11.6` through `3.11.7`. More versions
will be supported in the future.

If `serverImage` is not specified, a default image for the provided `serverType` and
`serverVersion` will automatically be used. If you want to use a different image, specify the image in the format `<qualified path>:<tag>`.

### Using a default image

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: dse
  serverVersion: 6.8.4

```

### Using a specific image

Cassandra:
```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: cassandra
  serverVersion: 3.11.7
  serverImage: private-docker-registry.example.com/cass-img/cassandra-with-mgmtapi:1a2b3c4d
```

DSE:
```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: dse
  serverVersion: 6.8.4
  serverImage: private-docker-registry.example.com/dse-img/dse:5f6e7d8c
```

## Configuring a NodePort service

A NodePort service may be requested by setting the following fields:

  networking:
    nodePort:
      native: 30001
      internode: 30002
 
The SSL versions of the ports may be requested:

  networking:
    nodePort:
      nativeSSL: 30010
      internodeSSL: 30020
      
If any of the nodePort fields have been configured then a NodePort service will be created that routes from the specified external port to the identically numbered internal port.  Cassandra will be configured to listen on the specified ports.

## Encryption

The operator automates the creation of key stores and trust stores
   for client-to-node and internode encryption. For each datacenter created with the operator,
   credentials are injected into the stateful set via secrets with the name `<datacenter-name>-keystore`.
   In order to use client-to-node or internode encryption, it is only necessary to reference the injected
   keystores from the cassandra parameters provided in the datacenter configuration. An example can be found
   in the [datacenter encryption test yamls](../../tests/testdata/encrypted-single-rack-2-node-dc.yaml#L27).

   Due to limitations of kubernetes stateful sets, the current strategy primarily focuses on internode encryption
   with ca-only verification (peer name verification is not currently available). Peer verification can be achieved with init containers,
   which may be able to leverage external certificate issuance architecture to enable per-node and per-client peer name verification.

   By storing the certificate authority in kubernetes secrets, it is possible to create secrets ahead of time from user-provided or organizational
   certificate authorities. It is also possible to leverage a single CA across multiple datacenters, by copying the secrets generated for one datacenter
   to the secondary datacenter prior to launching the secondary datacenter.

   It is possible to go from encrypted internode communications to unencrypted
   internode communications and the reverse, but this change as a rolling
   configuration is not currently supported, the entire cluster must be stopped
   and started to update these features.

# Using Your Cluster

## Connecting from inside the Kubernetes cluster

The operator makes a Kubernetes headless service available at
`<clusterName>-<datacenterName>-service`. Any CQL client inside the
Kubernetes cluster should be able to connect to
`cluster1-dc1-service.cass-operator` and use the nodes in a round-robin fashion
as contact points.

## Connecting from outside the Kubernetes cluster

Accessing the instances from CQL clients located outside the Kubernetes
cluster is an advanced topic, for which a detailed discussion is outside the
scope of this document.

Note that exposing Cassandra or DSE on the public internet with authentication disabled or
with the default username and password in place is extremely dangerous. It's
strongly recommended to protect your cluster with a network firewall during
deployment, and [secure the default superuser
account](https://docs.datastax.com/en/security/6.7/security/Auth/secCreateRootAccount.html)
before exposing any ports publicly.

## Scale up

The `size` parameter on the `CassandraDatacenter` determines how many server nodes
are present in the datacenter. To add more nodes, edit the YAML file from
the `Example Config` section above, and re-apply it precisely as before. The
operator will add pods to your datacenter, provided there are sufficient
Kubernetes worker nodes available.

For racks to act effectively as a fault-containment zone, each rack in the
cluster must contain the same number of instances.

## Scale down

The `size` parameter on the `CassandraDatacenter` resource can
also be decreased to scale down the datacenter by decommissioning nodes
evenly on racks. The `size` parameter cannot be smaller than the number of racks.

For a scale down to be successful, the remaining nodes in the cluster must
have enough storage capacity to absorb the data from the decommissioned nodes.
If this requirement is not met, the operator will log error messages displaying
the storage capacity requirements. The `Valid` condition on the datacenter will
be set to `FALSE` in the event of a failed scale down.

Just like with scaling up, the size should be reduced to a number that can be
divided evenly into the number of racks so that they can act effectively as a
fault-containment zone.

## Change server configuration

To change the database configuration, update the `CassandraDatacenter` and edit the
`config` section of the `spec`. The operator will update the config and restart
one node at a time in a rolling fashion.

## Multiple Datacenters in one Cluster

To make a multi-datacenter cluster, create two `CassandraDatacenter` resources and
give them the same `clusterName` in the `spec`.

_Note that multi-region clusters and advanced workloads are not supported, which
makes many multi-DC use-cases inappropriate for the operator._

# Maintaining Your Cluster

## Data Repair

The operator does not automate the process of performing traditional repairs on
keyspace ranges where the data has become inconsistent due to an instance
becoming unavailable in the past.

DSE provides
[NodeSync](https://www.datastax.com/2018/04/dse-nodesync-operational-simplicity-at-its-best),
a continuous background repair service that is declarative and
self-orchestrating. After creating your cluster, [Enable
NodeSync](https://docs.datastax.com/en/dse/6.7/dse-admin/datastax_enterprise/tools/dseNodesync/dseNodesyncEnable.html)
on all new tables.

Future releases may include integration with open source repair services for Cassandra clusters.

## Backup

The operator does not automate the process of scheduling and taking backups at
this time.

# Known Issues and Limitations

1. There is no facility for multi-region clusters. The operator functions
   within the context of a single Kubernetes cluster, which typically also
   implies a single geographic region.

