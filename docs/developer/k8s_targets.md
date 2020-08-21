# Using mage k8s targets for local development

## Overview

There are several mage targets exposed under the `k8s` namespace that can be
used to automate many setup/configuration steps for several k8s flavors.

Refer to the [mage documentation](./mage.md) if you do not have mage installed.

## Choosing a supported flavor

To see which flavors of k8s that we support with this set of mage targets, run:
```bash
mage k8s:listSupportedFlavors
```

A supported flavor can then be used by setting the `M_K8S_FLAVOR` env variable to that
flavor, for example:

```bash
M_K8S_FLAVOR=kind
```

If the `M_K8S_FLAVOR` is not specified, the default flavor becomes `k3d`

Note: Some flavors do not currently support every k8s mage target. Some targets
may print output explaining that manual steps may need to be taken in some cases,
and in others an error will be thrown.

## Viewing configurable env variables

There are certain env variables that can be used acrossed different k8s flavors,
but there could be many that are flavor-specific. Because of this, the `env` target
could print out different information depending on what flavor you choose.

Example usage:
```bash
M_K8S_FLAVOR=gke mage k8s:env
```

## Tool Installation

CLI tools for supported flavors can be installed by running the `installTool` target.

Example usage:
```bash
M_K8S_FLAVOR=kind mage k8s:installTool
```

## Setting up a cluster

A mage target can be invoked to stand up a new k8s cluster, along
with a sample datacenter and the Operator. This will attempt to
delete an existing cluster if there is one.

For Apache Cassandra:

```bash
M_K8S_FLAVOR=k3d mage k8s:setupCassandraCluster
```

For DSE:

```bash
M_K8S_FLAVOR=k3d mage k8s:SetupDSECluster
```

The resource files that get loaded into the cluster with this target are:
```bash
operator/deploy/kind/rancher-local-path-storage.yaml,
operator/deploy/role.yaml,
operator/deploy/role_binding.yaml,
operator/deploy/service_account.yaml,
operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml,
operator/deploy/operator.yaml,

# if using dse
operator/example-cassdc-yaml/dse-6.8.x/example-cassdc-minimal.yaml

# if using cassandra
operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml
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
mage k8s:deleteCluster
```

## Reloading the Docker operator image

For k8s flavors that run their workers inside of docker (KIND, k3d),
there is a mage target that will remove the existing operator image from every 
Docker container in the cluster and then rebuild/reload the operator Docker image 
into those worker containers.

This will need to be done after every code change of the operator code!

```bash
M_K8S_FLAVOR=k3d mage k8s:reloadOperator
```

Please make sure to stop any running pods of the operator before reloading the image.

## Shell configuration

After your k8s cluster has been created, you can configure kubectl to point to it.

### Bash shell
```bash
M_K8S_FLAVOR=k3d mage k8s:kubeconfig
```

Note: this target currently assumes that you are using bash. Different shells may
require manual configuration for the kubeconfig
