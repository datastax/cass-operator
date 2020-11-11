# Packaging for Operator Hub / Red Hat

## Tools
* `opm`
  1. Checkout a copy of [operator-framework/operator-registry](https://github.com/operator-framework/operator-registry)
  2. Build with `make build`
  3. Binary resides at `bin/opm`
* `k3d`

### Setup Test Cluster

Spin up local cluster for testing and install OLM
    
```console
k3d cluster create
kubectl create namespace test-operator
operator-sdk olm install

# Optional
kubectl delete catalogsource operatorhubio-catalog -n olm
```

### Unsupported Resource Types
The following types may not be included in the operator bundle

1. Namespace
2. Secret
3. ValidatingWebhookConfiguration
4. Service

## Common Issues
* `runAsUser` set to `999` in the deployment - OpenShift prefers to set a randomly assigned user at container start time. If this field is not left empty the user field must be set extremely high. 999 is too low.
* Prerequisite custom resources have not been included in the appropriate section of the operator metadata testing page
* The package has not been marked as published in the Red Hat repo.

# Upgrade Workflow

_Note see Red Hat's [Gitbook](https://redhat-connect.gitbook.io/partner-guide-for-red-hat-openshift-and-container/certify-your-operator/upgrading-your-operator)_

1. Copy an existing version as the base
   
    ```console
    OLD_VERSION=1.4.0
    NEW_VERSION=1.4.1
    cd operator/bundle
    cp -r $OLD_VERSION $NEW_VERSION
    cp bundle-$OLD_VERSION.Dockerfile bundle-$NEW_VERSION.Dockerfile
    ```
2. Rename files with version numbers
    
    ```console
    mv $NEW_VERSION/manifests/cass-operator.v$OLD_VERSION.clusterserviceversion.yaml $NEW_VERSION/manifests/cass-operator.v$NEW_VERSION.clusterserviceversion.yaml
    ```

3. Update version numbers in ClusterServiceVersion files
    
    ```console
    sed s/"$OLD_VERSION"/$NEW_VERSION/g $NEW_VERSION/manifests/cass-operator.v$NEW_VERSION.clusterserviceversion.yaml
    sed s/"$OLD_VERSION"/$NEW_VERSION/g bundle-$NEW_VERSION.Dockerfile
    ```

4. Copy in updated CRD

    ```console
    cp ../deploy/crds/* $NEW_VERSION/manifests/
    ```

5. Compare the old and new CSVs for differences and update `$NEW_VERSION/manifests/cass-operator.v$NEW_VERSION.lusterserviceversion.yaml`
   1. Update `metadata.annotations.containerImage` version
   2. Update `metadata.annotations.createdAt` datestamp
   3. Update `metadata.annotations.name` field
   4. Update `spec.install.spec.deployments[0].template.spec.containers[0].image` version
   5. Update `spec.customresourcedefinitions.owned[0].specDescriptors` to include any new spec fields. See [Descriptor](https://github.com/openshift/console/blob/master/frontend/packages/operator-lifecycle-manager/src/components/descriptors/reference/reference.md) [Documentation](https://github.com/openshift/console/tree/release-4.3/frontend/packages/operator-lifecycle-manager/src/components/descriptors).
   6. Update `spec.customresourcedefinitions.owned[0].statusDescriptors` to include any new status fields
   7. Update `spec.replaces` to replace previous version
   8. Update `spec.version` with new version value
6. Build bundle container and push to staging repo
    
    ```console
    docker build -t bradfordcp/cass-operator-bundle:$NEW_VERSION -f bundle-$NEW_VERSION.Dockerfile .
    docker push bradfordcp/cass-operator-bundle:$NEW_VERSION
    ```
7. Build local catalog index for testing
    
    ```console
    opm index add --bundles docker.io/bradfordcp/cass-operator-bundle:1.0.0,docker.io/bradfordcp/cass-operator-bundle:1.2.0,docker.io/bradfordcp/cass-operator-bundle:1.3.0,docker.io/bradfordcp/cass-operator-bundle:1.4.0,docker.io/bradfordcp/cass-operator-bundle:1.4.1 --tag docker.io/bradfordcp/catalog-index:1.4.1 -u docker
    docker tag bradfordcp/catalog-index:1.4.1 bradfordcp/catalog-index:latest
    docker push bradfordcp/catalog-index:latest
    ```
8. Add index as a [`CatalogSource`](olm/catalogsource.yaml) in k8s

    ```console
    kubectl apply -f docs/developer/olm/catalogsource.yaml
    ```
9. Verify packagemanifests are being pulled from index
    
    ```console
    kubectl describe packagemanifests cass-operator -n olm
    ```
10. Create an [`OperatorGroup`](olm/operatorgroup.yaml) to tell cass-operator where to watch for CassDC instances

    ```console
    kubectl apply -f docs/developer/olm/operatorgroup.yaml
    ```
11. Install prereqs that OLM doesn't handle
    
    ```console
    kubectl apply -f docs/developer/olm/prereqs.yaml
    ```
12. Install cass-operator with OLM via a [`Subscription`](olm/subscription.yaml)

    ```console
    kubectl apply -f docs/developer/olm/subscription.yaml
    ```
13. Check the `InstallPlan` to see the operator successfully installed

    ```console
    kubectl describe installplan -n test-operator | less
    ```

    If there is a failure, fix the issue locally, remove the `Subscription` and `CatalogSource`. Goto step 6 and repackage everything.
14. Check the operator is running

    ```console
    kubectl get deployments -n test-operator
    kubectl get pods -n test-operator
    ```
14. Provision a sample cassdc in the default namespace
15. Push to Red Hat
    
    ```console
    docker tag bradfordcp/cass-operator-bundle:$NEW_VERSION $REDHAT_REGISTRY/cass-operator-bundle:$NEW_VERSION
    docker push $REDHAT_REGISTRY/cass-operator-bundle:$NEW_VERSION
    ```
16. Red Hat automatically runs certification tests on push. These take 1-2 hours
17. Login to the Red Hat project and verify certification results. Repeat steps 5-7 until certification passes
18. Publish certified bundle
