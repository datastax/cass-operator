# cass-operator disaggregated storage solution

- [cass-operator disaggregated storage solution](#cass-operator-disaggregated-storage-solution)
  - [Proposed solution](#proposed-solution)
  - [Cass-operator Modifications](#cass-operator-modifications)
    - [Example Deployment](#example-deployment)
    - [Example Output](#example-output)

## Proposed solution

1. Update cass-operator to declare `storageConfig` per rack.
   A `storageConfig` has a `storageClassName` which means that we will be able to assign each rack a `StorageClass`.
2. Update `lb-csi-plugin` code to accept `failure-domain` field from StorageClass.Parameters and use it when invoking `LightOS.CreateVolume`
   This will enable us to create a one to one mapping between a `failure-domain` to a StorageClass to a rack in Cassandra.
3. Add support on LightOS API to specify `failure-domain` on `CreateVolume` call.
4. Push the `cass-operator` change to the Cassandra maintainers for review and acceptance

## Cass-operator Modifications

### Example Deployment

Added the following example files to demo the Rack assignment capability:

```bash
examples/lb-csi-cassandra/
├── default-cassandra-dc.yaml
├── lb-cassandra-dc.yaml
├── multi-rack-cass-dc.yaml
├── sc1.yaml
├── sc2.yaml
└── sc3.yaml
```

- `default-cassandra-dc.yaml`: the basic upstream example to show we support current configuration.
- `lb-cassandra-dc.yaml`: show new storageConfig settings to support disaggregated storage solution like `lb-csi-plugin`

### Example Output

You can see after running the modified operator and deploying `lb-cassandra-dc.yaml` CassandraDatacenter.

- `pod/cass-operator-845c77764-2hvd9` - modified operator
- `pod/lbcluster-lb-dc1-rack1-sts-0` - Cassandra rack1
- `pod/lbcluster-lb-dc1-rack1-sts-0` - Cassandra rack1
- `persistentvolumeclaim/server-data-lbcluster-lb-dc1-rack1-sts-0` - PVC for `rack1` **provisioned from `sc1`**
- `persistentvolumeclaim/server-data-lbcluster-lb-dc1-rack2-sts-0` - PVC for `rack2` **provisioned from `sc2`**

```bash
kubectl get -n cass-operator pods,pvc,sts,pv
NAME                                READY   STATUS    RESTARTS   AGE
pod/cass-operator-845c77764-2hvd9   1/1     Running   0          7m38s
pod/lbcluster-lb-dc1-rack1-sts-0    2/2     Running   0          6m10s
pod/lbcluster-lb-dc1-rack2-sts-0    2/2     Running   0          6m10s

NAME                                                             STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/server-data-lbcluster-lb-dc1-rack1-sts-0   Bound    pvc-ad918e69-438e-4819-9e61-e35b9dba833b   5Gi        RWO            sc1            6m10s
persistentvolumeclaim/server-data-lbcluster-lb-dc1-rack2-sts-0   Bound    pvc-d107c103-80c2-4970-aeca-23e578c165fc   5Gi        RWO            sc2            6m10s

NAME                                          READY   AGE
statefulset.apps/lbcluster-lb-dc1-rack1-sts   1/1     6m10s
statefulset.apps/lbcluster-lb-dc1-rack2-sts   1/1     6m10s

NAME                                                        CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                                    STORAGECLASS   REASON   AGE
persistentvolume/pvc-ad918e69-438e-4819-9e61-e35b9dba833b   5Gi        RWO            Delete           Bound    cass-operator/server-data-lbcluster-lb-dc1-rack1-sts-0   sc1                     6m10s
persistentvolume/pvc-d107c103-80c2-4970-aeca-23e578c165fc   5Gi        RWO            Delete           Bound    cass-operator/server-data-lbcluster-lb-dc1-rack2-sts-0   sc2                     4m20s
```
