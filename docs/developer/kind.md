# Using Kubernetes-in-Docker (KIND) for local development

## Overview

Kubernetes-in-Docker is a local kubernetes development and automated system 
that allows the provisioning of multi-node clusters.

Note that both the KIND control-plane node and the KIND worker nodes are 
running the exact same docker image named "kindest/node".

KIND uses the Controller Runtime Interface (CRI) instead of directly interacting 
with a container engine like Docker.  Therefore commands like "crictl" may be used 
inside the docker containers running kindest/node for administrative purposes.

For more information, see:

https://kind.sigs.k8s.io/
https://github.com/kubernetes-sigs/kind
https://kubernetes.io/docs/tasks/debug-application-cluster/crictl/

## Installation

KIND can be installed via mage, refer to the [mage documentation](./mage.md) if you need to install mage first.
Then, install kind with:

```bash
mage kind:install
```

## Setting up a cluster

You many need to manually pull the images referenced from `buildsettings.yaml` and `operator/deploy/kind/cassandradatacenter-one-rack-example.yaml`,
at the time of writing these are:

```bash
docker pull datastaxlabs/dse-k8s-server:6.8.0-20200316
docker pull datastaxlabs/dse-k8s-config-builder:0.9.0-20200316
docker pull datastaxlabs/apache-cassandra-with-mgmtapi:3.11.6-20200316
docker pull datastaxlabs/dse-k8s-operator:0.9.0-20200316
```

A mage target can then be invoked to stand up a new KIND cluster, along
with a sample datacenter and the Operator. This will
delete an existing kind cluster if there is one.

For Apache Cassandra:

```bash
mage kind:setupCassandraCluster
```

For DSE:

```bash
mage kind:SetupDSECluster
```

The resource files that get loaded into the cluster with this target are:
```
operator/deploy/kind/rancher-local-path-storage.yaml,
operator/deploy/role.yaml,
operator/deploy/role_binding.yaml,
operator/deploy/service_account.yaml,
operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml,
operator/deploy/operator.yaml,
operator/deploy/kind/cassandradatacenter-one-rack-example.yaml,
```

To check the cluster status:
```bash
kubectl get pods
kubectl describe pod clstr-dtcntr-r0-sts-0
```

To connect to the cluster:
```bash
kubectl exec -it clstr-dtcntr-r0-sts-0 -- /bin/bash
```
To invoke cqlsh:
```bash
cqlsh /tmp/dse.sock
```

To delete the cluster:
```bash
mage kind:deleteCluster
```

## Reloading the Docker operator image

KIND does not currently provide a way to replace an existing Docker image.
There is a mage target that will remove the existing operator image from every 
Docker container in the KIND cluster and then reload the most recent copy of the 
operator Docker image into KIND.

This will need to be done after every rebuild of the Docker operator image!

```bash
mage kind:reloadOperator
```

Please make sure to stop any running pods of the operator before reloading the image.

## Shell configuration

After your KIND cluster has been created, you can configure kubectl to point to it.

### Bash shell
```bash
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
```
### Fish shell
```bash
export KUBECONFIG=(kind get kubeconfig-path --name="kind")
```
