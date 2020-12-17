# cass-operator integration tests

[Ginkgo](https://onsi.github.io/ginkgo/) is used alongside kubectl
for integration testing of the operator.

## Prerequisites
Install mage using the instructions
[here](https://github.com/magefile/mage#installation)

Install kubectl using the instructions
[here](https://kubernetes.io/docs/tasks/tools/install-kubectl) 

## Running the tests
The tests themselves expect a running k8s cluster, with at least 6 worker nodes,
and kubectl to be configured to point at the cluster.

You can either stand up your own cluster and point kubectl to it yourself, or use
a mage target to bootstrap and configure a cluster for you.

Refer to the [k8s targets](../docs/developer/k8s_targets.md) for more information on
supported k8s flavors.

### Using preconfigured targets for cluster management

To let a mage bootstrap a new cluster, run all of the tests,
and then tear down the cluster at the end, run the `k8s:runIntegTests` target
with the desired k8s flavor specified:

#### Using a k3d cluster
```
M_K8S_FLAVOR=k3d mage k8s:runIntegTests
```

#### Using a KIND cluster
```
M_K8S_FLAVOR=kind mage k8s:runIntegTests
```

If you just want to have the cluster stood up and configured for you, but
not automatically run the tests or tear down, run:

#### Using a k3d cluster
```
M_K8S_FLAVOR=k3d mage k8s:setupEmptyCluster
```

### Using a custom Docker registry for the operator image

If M_DOCKER_USERNAME, M_DOCKER_PASSWORD, and M_DOCKER_SERVER environment variables are defined, then they will be used to create an image pull secret.  This secret will automatically be used by the integration tests for pulling the Docker image of the operator.

This should be used in conjunction with the M_OPERATOR_IMAGE environment variable to select a specific image.  Note that the M_OPERATOR_IMAGE value should include the name of the custom registry.

Example:

```console
export M_DOCKER_USERNAME=USERNAME
export M_DOCKER_PASSWORD=ACCESSTOKEN
export M_DOCKER_SERVER="docker.pkg.github.com"
export M_OPERATOR_IMAGE="docker.pkg.github.com/datastax/cass-operator/operator:latest-ubi"
```

Replace USERNAME with the Github username and ACCESSTOKEN with a Github access token in the above commands.

Note: The automatically created image pull secret will be named "imagepullsecret", and it will be removed at the end of each test when the test namespace is deleted.

### Kicking off the tests on an existing cluster
To kick off all integration tests against the cluster that your kubectl
is currently configured against, run:
```
mage integ:run
```

### Running a single test
If you only want to run a single test, you can specify it's parent directory
in an environment variable called `M_INTEG_DIR`:
```
M_INTEG_DIR=scale_up mage integ:run
```

### Running a test with a custom operator image
If you are building the operator image locally, you will want to use it for integration tests.

```
# This will create a datastax/cass-operator:latest image.
$ mage operator:build

# Tag the image with the commit hash. T
# The Docker Hub org for the example is bob.
$ docker tag docker.io/datastax/cass-operator:latest bob/cass-operator:6208cb4a8b1c

# If you are running tests against a remote cluster you will 
# need to push your image to registry.
$ docker push bob/cass-operator:6208cb4a8b1c

# Run a single test using the image.
$ M_INTEG_DIR=scale_up M_OPERATOR_IMAGE=docker.io/bob/cass-operator:6208cb4a8b1c mage integ:run
```

## Test structure and design
Our tests are structured such that there is a single Ginkgo test suite
per integration test. This is done in order to provide the maximum amount
of flexibility when running locally and in Jenkins. The main benefits we
get from this structure are:

* We can separate the output streams from each test
* This allows for better parallelization
* It becomes trivial to run just a single test if needed

We also structure each test so that the steps live inside of a single spec.
This is done to support parallel test running, as well as halting the test on
failure.

After each test, we automatically clean up and delete the namespace that it used. 
It is important for tests to use a namespace that begins with `test-` in order for
cleanup logic to work correctly during CI builds.

### Logging/Debugging
When using the provided `NamespaceWrapper` struct and accompanying functions to
execute test steps, we automatically dump logs for the current namespace after
each step. This provides a snapshot in time for easy inspection of the cluster
state throughout the life of the test.

After the test is done running, whether it was successful or not, we do a
final log dump for the entire cluster.

By default, we dump logs out at:
```
build/kubectl_dump/<test name>/<date/time stamp>/<steps>
```
