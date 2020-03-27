# Deploying the operator and making a Cassandra cluster with it

## Make a Kubernetes cluster

For the rest of this README I'm going to assume your `kubectl` is working with a GKE cluster on the receiving end of the API commands. Most of this should hold up for other k8s environments.

## Create a namespace

```
kubectl create ns optest
```

## Create a secret so you can use the DS internal Artificatory (datastax-docker.jfrog.io) for docker images

```
kubectl -n optest create secret docker-registry cass-operator-artifactory-secret --docker-server=datastax-docker.jfrog.io --docker-username=USERNAME --docker-password=PASSWORD
```

Feel free to ask Jim or some folks in #cloud-eng on Slack to get the right values to for that secret.

## Apply a bunch of yamls

```
kubectl -n optest apply -f role.yaml
kubectl -n optest apply -f role_binding.yaml
kubectl -n optest apply -f service_account.yaml
kubectl -n optest apply -f crds/datastax_v1alpha1_dsedatacenter_crd.yaml
```

This created a `Role`, `RoleBinding`, `ServiceAccount` (using that secret to pull Docker images),
and the `DseDatacenter` CRD.

## Deploy the Operator

Edit `operator.yaml` and swap the string REPLACE_IMAGE with the operator build you want to use. For example, `datastax-docker.jfrog.io/cass-operator/operator:master.2290d5ab557b97f6c736fee5911a62dea8c63d29`

Then, deploy the operator.

```
kubectl -n optest apply -f operator.yaml
```

## Create your StorageClass

It's up to the k8s admin to create a `StorageClass` the operator can use to set up volumes for the DSE cluster. I've left an example one for GKE us-west2 clusters using 100GB SSD volumes in `gke/storage.yaml`.

```
kubectl -n optest apply -f gke/storage.yaml
```

## Create a Cassandra Cluster

At this point the only step left is to create a `CassandraDatacenter` and wait for the operator to provision the pods and volumes. I've left an example that uses the `StorageClass` created in the previous step.

```
kubectl -n optest apply -f gke/gke-example-dc.yaml
```
