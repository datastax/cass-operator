# Introduction

Cass Operator simplifies the process of deploying
and managing Cassandra or DSE in a Kubernetes cluster.

# Install the operator

## Prerequisites

1. A Kubernetes cluster. Kubernetes v1.15 is recommended, but Kubernetes
   v1.13 has been tested and works provided the line containing
   `x-kubernetes-preserve-unknown-fields: true` is deleted from
   `cass-operator-manifests.yaml`.
2. The ability to download images from Docker Hub from within the Kubernetes
   cluster.

## Create a namespace

cass-operator is built to watch over pods running Casandra or DSE in a Kubernetes
namespace. Create a namespace for the cluster with:

```shell
$ kubectl create ns my-db-ns
```

For the rest of this guide, we will be using the namespace `my-db-ns`. Adjust
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
$ kubectl -n my-db-ns apply -f ./server-storage.yaml

$ kubectl -n my-db-ns get storageclass
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
[operator-manifests YAML](cass-operator-manifests.yaml).
_Note the operator does not require `cluster-admin` privileges, only the user
defining the CRD requires those permissions._

Apply the manifest above, and wait for the deployment to become ready. You can
watch the progress by getting the list of pods for the namespace, as
demonstrated below:

```shell
$ kubectl -n my-db-ns apply -f ./cass-operator-manifests.yaml

$ kubectl -n my-db-ns get pod
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

Cassandra is rack-aware, and the `racks` parameter will configure the operator to
set up pods in a rack aware way. Note the Kubernetes worker nodes must have
labels matching `failure-domain.beta.kubernetes.io/zone`. Racks must have
identifiers. In this guide we will use `r1`, `r2`, and `r3`.

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

$ kubectl -n my-db-ns get secret cluster1-superuser
NAME                       TYPE                                  DATA   AGE
cluster1-superuser         Opaque                                2      13m

$ kubectl -n my-db-ns get secret cluster1-superuser -o yaml
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
apiVersion: cassandra.datastax.com/v1alpha2
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  superuserSecretName: superuser-secret
```

## Specifying version and image

With the release of the operator v0.4.0 comes a new way to specify
which version of Cassandra or  DSE and image you want to use. From within the config yaml
for your `CassandraDatacenter` resource, you can use the `serverType`, `imageVersion`, and `serverImage`
spec properties.

`serverType` is required and must be either `dse` or `cassandra`. `imageVersion` is also required,
and the supported version for DSE is `6.8.0` and for Cassandra it is `3.11.6`. More versions
will be supported in the future.

If `serverImage` is not specified, a default image for the provided `serverType` and
`imageVersion` will automatically be used. If you want to use a different image, specify the image in the format `<qualified path>:<tag>`.

### Using a default image

```yaml
apiVersion: cassandra.datastax.com/v1alpha2
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: dse
  imageVersion: 6.8.0

```

### Using a specific image

Cassandra:
```yaml
apiVersion: cassandra.datastax.com/v1alpha2
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: cassandra
  imageVersion: 3.11.6
  serverImage: datastaxlabs/apache-cassandra-with-mgmtapi:3.11.6-20200316
```

DSE:
```yaml
apiVersion: cassandra.datastax.com/v1alpha2
kind: CassandraDatacenter
metadata:
  name: dtcntr
spec:
  serverType: dse
  imageVersion: 6.8.0
  serverImage: datastaxlabs/dse-k8s-server:6.8.0-20200316
```

## Example Config

The following example illustrates a `CassandraDatacenter` resource.

```yaml
apiVersion: cassandra.datastax.com/v1alpha2
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverImage: datastaxlabs/dse-k8s-server:6.8.0-20200316
  serverType: dse
  imageVersion: 6.8.0
  managementApiAuth:
    insecure: {}
  size: 3
  storageConfig:
    cassandraDataVolumeClaimSpec:
      storageClassName: server-storage
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 10Gi
  racks:
    - name: r1
      # zone: us-central1-a
    - name: r2
      # zone: us-central1-b
    - name: r3
      # zone: us-central1-f
  config:
    dse-yaml:
      authentication_options:
        enabled: False
    #cassandra-yaml:
    #  num_tokens: 32
    #jvm-server-options:
    #  initial_heap_size: "16g"
    #  max_heap_size: "16g"
    #10-write-prom-conf:
    #  enabled: True
```

Consider customizing the example above to suit your requirements, and save it as
`cluster1-dc1.yaml`. Apply this file via `kubectl` and watch the list of pods as
the operator deploys them. Completing a deployment may take several minutes per
node. The best way to track the operator's progress is by using
`kubectl -n my-db-ns describe caasdc dc1` and checking the `status` and events.

```shell
$ kubectl -n my-db-ns apply -f ./cluster1-dc1.yaml

$ kubectl -n my-db-ns get pods
NAME                            READY   STATUS    RESTARTS   AGE
cass-operator-f74447c57-kdf2p   1/1     Running   0          13m
gke-cluster1-dc1-r1-sts-0       1/1     Running   0          5m38s
gke-cluster1-dc1-r2-sts-0       1/1     Running   0          42s
gke-cluster1-dc1-r3-sts-0       1/1     Running   0          6m7s

$ kubectl -n my-db-ns describe cassdc dc1
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

# Using Your Cluster

## Connecting from inside the Kubernetes cluster

The operator makes a Kubernetes headless service available at
`<clusterName>-<datacenterName>-service`. Any CQL client inside the
Kubernetes cluster should be able to connect to
`cluster1-dc1-service.my-db-ns` and use the nodes in a round-robin fashion
as contact points.

## Connecting from outside the Kubernetes cluster

Accessing the instances from CQL clients located outside the Kubernetes
cluster is an advanced topic, for which a detailed discussion is outside the
scope of this document.

Note that exposing Cassandra or DSE on the public internet with authentication disabled or
with the default username and password in place is extremely dangerous. Its
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

1. The DSE Operator is not recommended nor supported for production use.  This
   release is an early preview of an unfinished product intended to allow proof
   of concept deployments and to facilitate early customer feedback into the
   software development process.
2. The operator is compatible with DSE 6.8.0 and above. It will not function
   with prior releases of DSE. Furthermore, version 0.4.1 of the operator is
   compatible only with a specific DSE docker image co-hosted in the labs
   [Docker Hub
   repository](https://cloud.docker.com/u/datastaxlabs/repository/docker/datastaxlabs/dse-k8s-server).
   Other labs releases of DSE 6.8.0 will not function with the operator.
3. The operator is compatible with DSE and the Cassandra workload only. It does
   not support DDAC or Advanced Workloads like Analytics, Graph, and Search.
4. There is no facility for multi-region DSE clusters. The operator functions
   within the context of a single Kubernetes cluster, which typically also
   implies a single geographic region.
5. The operator does not automate the repair or decommission/bootstrap of nodes
   that lose access to their data volume. With NodeSync enabled, the DSE
   instance should recover over time. The operator will not be aware that the
   DSE instance is unable to serve traffic and might make incorrect
   `podDisruptionBudget` decisions. Due to this limitation, it's not recommended
   to use local volumes.
6. The operator does not automate the creation of key stores and trust stores
   for client-to-node and internode encryption.

# Changelog

## v0.9.0

* KO-146 Create a secret for superuser creation if one is not provided.
* KO-288 The operator can provision Cassandra clusters using images from
  https://github.com/datastax/management-api-for-apache-cassandra and the primary
  CRD the operator works on is a `v1alpha2` `cassandra.datastax.com/CassandraDatacenter`
* KO-210 Certain `CassandraDatacenter` inputs were not rolled out to pods during
  rolling upgrades of the cluster. The new process considers everything in the
  statefulset pod template.
* KO-276 Greatly improved integration tests on real KIND / GKE Kubernetes clusters
  using Ginkgo.
* KO-223 Watch fewer Kubernetes resources.
* KO-232 Following best practices for assigning seed nodes during cluster start.
* KO-92 Added a container that tails the system log.

## v0.4.1
* KO-190 Fix bug introduced in v0.4.0 that prevented scaling up or deleting
  datacenters.
* KO-177 Create a headless service that includes pods that are not ready. While
  this is not useful for routing CQL traffic, it can be helpful for monitoring
  infrastructure like Prometheus that would like to attempt to collect metrics
  from pods even if they are unhealthy, and which can tolerate connection
  failure.

## v0.4.0
* KO-97  Faster cluster deployments
* KO-123 Custom CQL super user. Clusters can now be provisioned without the
  publicly known super user `cassandra` and publicly known default password
  `cassandra`.
* KO-42  Preliminary support for DSE upgrades
* KO-87  Preliminary support for two-way SSL authentication to the DSE
  management API. At this time, the operator does not automatically create
  certificates.
* KO-116 Fix pod disruption budget calculation. It was incorrectly calculated
  per-rack instead of per-datacenter.
* KO-129 Provide `allowMultipleNodesPerWorker` parameter to enable testing
  on small k8s clusters.
* KO-136 Rework how DSE images and versions are specified.

## v0.3.0
* Initial labs release.
