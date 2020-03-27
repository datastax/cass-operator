# Minikube Quickstart

```bash
minikube start --vm-driver=virtualbox --cpus=2 --memory=8192

eval $(minikube docker-env)

mage operator:buildDocker

kubectl create secret docker-registry cass-operator-registry-secret \
  --docker-server="<DOCKER_REGISTRY_HOST> "\
  --docker-username="<DOCKER_REGISTRY_USERNAME>" \
  --docker-password="<DOCKER_REGISTRY_PASSWORD>"

kubectl apply -f operator/deploy/minikube/storage.yaml
kubectl apply -f operator/deploy/role.yaml
kubectl apply -f operator/deploy/role_binding.yaml
kubectl apply -f operator/deploy/service_account.yaml
kubectl apply -f operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml
kubectl apply -f operator/deploy/operator.yaml
kubectl apply -f operator/deploy/minikube/minikube-one-rack-example.yaml

watch -n1 kubectl get pods
```

## Note for OSX: "watch" replacement

"watch" is not available by default in OSX.  The following should provide equivalent functionality:

```bash
while :; do clear; kubectl get pods; sleep 1; done
```

# Minikube Overview

https://github.com/kubernetes/minikube

Minikube is a de-facto standard for operator and kubernetes development.  Note that Minikube is limited to one Kubernetes node.  KIND does not have this limitation.

Multiple back-ends are supported, but the virtualbox back-end seems to be the most stable.

# Starting minikube

Adjust the cpus and memory to suit the capabilities of your local environment.

``` bash
minikube start --vm-driver=virtualbox --cpus=2 --memory=8192
```

# Running the operator inside of minikube

If the operator is run locally, there are DNS routing issues.  Therefore it is recommended to run the operator inside of minikube.

# Load operator Docker image into minikube

The easiest way to test minikube changes is to:

1. Start minikube

``` bash
minikube start --vm-driver=virtualbox --cpus=2 --memory=8192
```

2. evaluate the following in your shell of choice:

For bash:

```bash
eval $(minikube docker-env)
```

For fish:

```bash
eval (minikube docker-env)
```

3. Run the mage buildDocker target

```bash
mage operator:buildDocker
```

The freshly built docker image will be automatically loaded in minikube.

4. Add the Datastax artifactory secret

This is necessary to be able to pull DSE and configbuilder images:

```bash
kubectl create secret docker-registry cass-operator-registry-secret \
  --docker-server="<DOCKER_REGISTRY_HOST> "\
  --docker-username="<DOCKER_REGISTRY_USERNAME>" \
  --docker-password="<DOCKER_REGISTRY_PASSWORD>"
```

5. Add storage class and role information

```bash
kubectl apply -f operator/deploy/minikube/storage.yaml
kubectl apply -f operator/deploy/role.yaml
kubectl apply -f operator/deploy/role_binding.yaml
kubectl apply -f operator/deploy/service_account.yaml
```

6. Load the CRD definition

```bash
kubectl apply -f operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml
```

7. Start a copy of the operator in minikube

```bash
kubectl apply -f operator/deploy/operator.yaml
```

8. Load an example CRD

```bash
kubectl apply -f operator/deploy/minikube/minikube-one-rack-example.yaml
```

9. Watch the cluster come up

```bash
watch -n1 kubectl get pods
```

# Turn off the operator

First remove the CRD example and then turn off the operator:

```bash
kubectl delete -f operator/deploy/minikube/minikube-one-rack-example.yaml
kubectl delete -f operator/deploy/operator.yaml
```
