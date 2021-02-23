# Changelog

## v1.6.0

Features:
* Upgrade to Go 1.14 [#347](https://github.com/datastax/cass-operator/issues/347)
* Add support for specifying additional PersistentVolumeClaims [#327](https://github.com/datastax/cass-operator/issues/327)
* Add support for specify rack labels [#292](https://github.com/datastax/cass-operator/issues/292)

Bug fixes:
* Retry decommission to prevent cluster from getting stuck in decommission state [#356](https://github.com/datastax/cass-operator/pull/356)
* Make the busybox image configurable [#339](https://github.com/datastax/cass-operator/pull/339)
* Incorrect volume mounts are created when adding an init container with volume mounts 
  [#309](https://github.com/datastax/cass-operator/issues/309)

Docs/tests:
* Introduce integration with [Go Report Card](https://goreportcard.com/) [#346]
  (https://github.com/datastax/cass-operator/pull/346/files)
* Add more examples for running integration tests [#338](https://github.com/datastax/cass-operator/pull/338/files) 
 
## v1.5.1

Bug fixes:
* Fixed reconciling logic in VMware k8s environments [#361](https://github.com/datastax/cass-operator/commit/821e58e6b283ef77b25ac7791ae9b60fbc1f69a1) [#342](https://github.com/datastax/cass-operator/commit/ddc3a99c1b38fb54ba293aeef74f878a3bb7e07e)
* Retry decommission if worker nodes are too slow to start it [#356](https://github.com/datastax/cass-operator/commit/475ea74eda629fa0038214dce5969a7f07d48615)


## v1.5.0

Features:
* Allow configuration of the system.log tail-er [#311](https://github.com/datastax/cass-operator/commit/14b1310164e57546cdf1995d44835b63f2dee471)
* Update Kubernetes support to cover 1.15 to 1.19 [#296](https://github.com/datastax/cass-operator/commit/a75754989cd29a38dd6406fdb75a5bad010f782d) [#304](https://github.com/datastax/cass-operator/commit/f34593199c06bb2536d0957eaff487153e36041a)
* Specify more named ports for advanced workloads, and additional ports for ClusterIP service [#289](https://github.com/datastax/cass-operator/commit/b0eae79dae3dc1466ace18b2008d3de81622db5d) [#291](https://github.com/datastax/cass-operator/commit/00c02be24779262a8b89f85e64eb02efb14e2d15)
* Support WATCH_NAMESPACE=* to watch all namespaces [#286](https://github.com/datastax/cass-operator/commit/f856de1cbeafee5fe4ad7154930d20ca743bd9cc)
* Support arbitrary Cassandra and DSE versions, using a regex for validation [#271](https://github.com/datastax/cass-operator/commit/71806eaa1c30e8555abeca9f4db12490e8658e1a)
* Support running cassandra as a non-root cassandra user [#275](https://github.com/datastax/cass-operator/commit/4006e56f6dc2ac2b7cfc4cc7012cad5ee50dbfe8)
* Always gracefully drain Cassandra nodes before pod termination, and make terminationGracePeriodSeconds configurable [#269](https://github.com/datastax/cass-operator/commit/3686fd112e1363b1d1160664d92c2a54b368906f)
* Add canaryUpgradeCount to support canary upgrade of a single node [#258](https://github.com/datastax/cass-operator/commit/f73041cc6f3bb3196b49ffaa615e4d1107350330)
* Support merging all user customizations into the Cassandra podTemplateSpec [#263](https://github.com/datastax/cass-operator/commit/b1851d9ebcd4087a210d8ee3f83b6e5a1ab49516)
* DSE 6.8.4 support [#257](https://github.com/datastax/cass-operator/commit/d4d4e3a49ece233457e0e1deebc9153241fe97be)
* Add arm64 support [#238](https://github.com/datastax/cass-operator/commit/f1f237ea6d3127264a014ad3df79d15964a01206)
* Support safely scaling down a datacenter, decommissioning Cassandra nodes [#242](https://github.com/datastax/cass-operator/commit/8d0516a7b5e7d1198cf8ed2de7b3968ef9e660af) [#265](https://github.com/datastax/cass-operator/commit/e6a420cf12c9402cc038f30879bc1ee8556f487a)
* Add support to override the default registry for all container images [#228](https://github.com/datastax/cass-operator/commit/f7ef6c81473841e42a9a581b44b1f33e1b4ab4c1)
* Better helm chart support for branch / master builds on private Docker registries [#236](https://github.com/datastax/cass-operator/commit/3516aa1993b06b405acf51476c327bec160070a1)
* Support for specific reconciliation logic in VMware k8s environments [#204](https://github.com/datastax/cass-operator/commit/e7d20fbb365bc7679856d2c84a1af70f73167b57) [#206](https://github.com/datastax/cass-operator/commit/a853063b9f08eb1a64cb248e23b5a4a5db92a5ac) [#203](https://github.com/datastax/cass-operator/commit/10ff0812aa0138a2eb04c588fbe625f6a69ce833) [#224](https://github.com/datastax/cass-operator/commit/b4d7ae4b0da149eb896bacfae824ed6547895dbb) [#259](https://github.com/datastax/cass-operator/commit/a90f03c17f3e9675c1ceff37a2a7c0c0c85950a1)  [#288](https://github.com/datastax/cass-operator/commit/c7c159d53d34b390e7966477b0fc2ed2f5ec333e)

Bug fixes:
* Increase default init container CPU for better startup performance [#261](https://github.com/datastax/cass-operator/commit/0eece8718b7a7a48868c260e88684c0f98cdc05b)
* Re-enable the most common quiet period [#253](https://github.com/datastax/cass-operator/commit/67217d8fc384652753331f5a1ae9cadd45ebc0cb)
* Added label to all-pods service to narrow metrics scrape selection [#277](https://github.com/datastax/cass-operator/commit/193afa5524e85a90cea84afe8167b356f67d365a)


Docs/tests:
* Add test for infinite reconcile [#220](https://github.com/datastax/cass-operator/commit/361ef2d5da59cd48fab7d4d58e3075051515b32c)
* Added keys from datastax/charts and updated README [#221](https://github.com/datastax/cass-operator/commit/5a56e98856d0c483eb4c415bd3c8b8292ffffe4b)
* Support integration tests with k3d v3 [#248](https://github.com/datastax/cass-operator/commit/06804bd0b3062900c10a4cad3d3124e83cf17bf0)
* Default to KIND for integration tests [#252](https://github.com/datastax/cass-operator/commit/dcaec731c5d0aefc2d2249c32592a275c42a46f0)
* Run oss/dse integration smoke tests in github workflow [#267](https://github.com/datastax/cass-operator/commit/08ecb0977e09b547c035e280b99f2c36fa1d5806)


## v1.4.1

Features:
* Use Cassandra 4.0-beta1 image [#237](https://github.com/datastax/cass-operator/commit/085cdbb1a50534d4087c344e1e1a7b28a5aa922e)
* Update to cass-config-builder 1.0.3 [#235](https://github.com/datastax/cass-operator/commit/7c317ecaa287f175e9556b48140b648d2ec60aa2)
* DSE 6.8.3 support [#233](https://github.com/datastax/cass-operator/commit/0b9885a654ab09bb4144934b217dbbc9d0869da3)

Bug fixes:
* Fix for enabling DSE advanced workloads [#230](https://github.com/datastax/cass-operator/commit/72248629b7de7944fe9e7e262ef8d3c7c838a8c6)

## v1.4.0

Features:
* Cassandra 3.11.7 support [#209](https://github.com/datastax/cass-operator/commit/ecf81573948cf239180ab62fa72b91c9a8354a4e)
* DSE 6.8.2 support [#207](https://github.com/datastax/cass-operator/commit/4d566f4a4d16c975919726b95a51c3be8729bf3e)
* Configurable resource requests and limits for init and system-logger containers. [#184](https://github.com/datastax/cass-operator/commit/94634ba9a4b04f33fe7dfee539d500e0d4a0c02f)
* Add quietPeriod and observedGeneration to the status [#190](https://github.com/datastax/cass-operator/commit/8cf67a6233054a6886a15ecc0e88bd6a9a2206bd)
* Update config builder init container to 1.0.2 [#193](https://github.com/datastax/cass-operator/commit/811bca57e862a2e25c1b12db71e0da29d2cbd454)
* Host network support [#186](https://github.com/datastax/cass-operator/commit/86b32ee3fc21e8cd33707890ff580a15db5d691b)
* Helm chart option for cluster-scoped install [#182](https://github.com/datastax/cass-operator/commit/6c23b6ffe0fd45fa299540ea494ebecb21bc4ac9)
* Create JKS for internode encryption [#156](https://github.com/datastax/cass-operator/commit/db8d16a651bc51c0256714ebdda2d51485c5d9fa)
* Headless ClusterIP service for additional seeds [#175](https://github.com/datastax/cass-operator/commit/5f7296295e4dd7fda6b7ce7e2faca2b9efe2e414)
* Operator managed NodePort service [#177](https://github.com/datastax/cass-operator/commit/b2cf2ecf05dfc240d670f13dffc8903fe14bb052)
* Experimental ability to run DSE advanced workloads [#158](https://github.com/datastax/cass-operator/commit/ade4246b811a644ace75f9f561344eb815d43d52)
* More validation logic in the webhook [#165](https://github.com/datastax/cass-operator/commit/3b7578987057fd9f90b7aeafea1d71ebbf691984)


Bug fixes:
* Fix watching CassDC to not trigger on status update [#212](https://github.com/datastax/cass-operator/commit/3ae79e01398d8f281769ef079bff66c3937eca24)
* Enumerate more container ports [#200](https://github.com/datastax/cass-operator/commit/b0c004dc02b22c34682a3602097c1e09b6261572)
* Resuming a stopped CassDC should not use the ScalingUp condition [#198](https://github.com/datastax/cass-operator/commit/7f26e0fd532ce690de282d1377dd00539ea8c251)
* Idiomatic usage of the term "internode" [#197](https://github.com/datastax/cass-operator/commit/62993fa113053075a772fe532c35245003912a2f)
* First-seed-in-the-DC logic should respect additionalSeeds [#180](https://github.com/datastax/cass-operator/commit/77750d11c62f2c3043f1088e377b743859c3be96)
* Use the additional seeds service in the config [#189](https://github.com/datastax/cass-operator/commit/4aaaff7b4e4ff4df626aa12f149329b866a06d35)
* Fix operator so it can watch multiple or all namespaces [#173](https://github.com/datastax/cass-operator/commit/bac509a81b6339219fe3fc313dbf384653563c59)


Docs/tests:
* Encryption documentation [#196](https://github.com/datastax/cass-operator/commit/3a139c5f717a165c0f32047b7813b103786132b8)
* Fix link to sample-cluster-sample-dc.yaml [#191](https://github.com/datastax/cass-operator/commit/28039ee40ac582074a522d2a13f3dfe15350caac)
* Kong Ingress Documentation [#160](https://github.com/datastax/cass-operator/commit/b70a995eaee988846e07f9da9b4ab07e443074c2)
* Adding AKS storage example [#164](https://github.com/datastax/cass-operator/commit/721056a435492e552cd85d18e38ea85569ba755f)
* Added ingress documentation and sample client application to docs [#140](https://github.com/datastax/cass-operator/commit/4dd8e7c5d53398bb827dda326762c2fa15c131f9)


## v1.3.0
* Add DSE 6.8.1 support, and update to config-builder 1.0.1 [#139](https://github.com/datastax/cass-operator/commit/8026d3687ee6eb783743ea5481ee51e69e284e1c)
* Experimental support for Cassandra Reaper running in sidecar mode [#116](https://github.com/datastax/cass-operator/commit/30ac85f3d71886b750414e90476c42394d439026)
* Support using RedHat universal base image containers [#95](https://github.com/datastax/cass-operator/commit/6f383bd8d22491c5a784611620e1327dafc3ffae)
* Provide an easy way to specify additional seeds in the CRD [#136](https://github.com/datastax/cass-operator/commit/0125b1f639830fad31f4b0b1b955ac991212fd16)
* Unblocking Kubernetes 1.18 support [#132](https://github.com/datastax/cass-operator/commit/b8bbbf15394119cbbd604aa40fdb9224a9f312cd)
* Bump version of Management API sidecar to 0.1.5 [#129](https://github.com/datastax/cass-operator/commit/248b30efe797d0656f2fc5c8e96dc3c431ab9a32)
* No need to always set LastRollingRestart status [#124](https://github.com/datastax/cass-operator/commit/d0635a2507080455ed252a26252a336a96252bc9)
* Set controller reference after updating StatefulSets, makes sure StatefulSets are cleaned up on delete [#121](https://github.com/datastax/cass-operator/commit/f90a4d30d37fa8ace8119dc7808fd7561df9270e)
* Use the PodIP for Management API calls [#112](https://github.com/datastax/cass-operator/commit/dbf0f67aa7c3831cd5dacc52b10b4dd1c59a32d1)
* Watch secrets to trigger reconciling user and password updates [#109](https://github.com/datastax/cass-operator/commit/394d25a6d6ec452ecd1667f3dca40b7496379eea)
* Remove NodeIP from status [#96](https://github.com/datastax/cass-operator/commit/71ed104a7ec642e13ef27bafb6ac1a6c0a28a21e)
* Add ability to specify additional Cassandra users in CassandraDatacenter [#94](https://github.com/datastax/cass-operator/commit/9b376e8be93976a0a344bcda2e417aa90dd9758f)
* Improve validation for webhook configuration [#103](https://github.com/datastax/cass-operator/commit/6656d1a2fd9cdec1fe495c28dd3fbac9617341f6)

## v1.2.0
* Support for several k8s versions in the helm chart [#97](https://github.com/datastax/cass-operator/commit/9d76ad8258aa4e1d4893a357546de7de80aef0a0)
* Ability to roll back a broken upgrade / configuration change [#85](https://github.com/datastax/cass-operator/commit/86b869df65f8180524dc12ff11502f6f6889eef5)
* Mount root as read-only and temp dir as memory emptyvol [#86](https://github.com/datastax/cass-operator/commit/0474057e8339da4f89b2e901ab697f10a2184d78)
* Fix managed-by label [#84](https://github.com/datastax/cass-operator/commit/39519b8bae8795542a5fb16a844aeb55cf3b2737)
* Add sequence diagrams [#90](https://github.com/datastax/cass-operator/commit/f1fe5fb3e07cec71a2ba0df8fabfec2b7751a95b)
* Add PodTemplateSpec in CassDC CRD spec, which allows defining a base pod template spec [#67](https://github.com/datastax/cass-operator/commit/7ce9077beab7fb38f0796c303c9e3a3610d94691)
* Support testing with k3d [#79](https://github.com/datastax/cass-operator/commit/c360cfce60888e54b10fdb3aaaa2e9521f6790cf)
* Add logging of all events for more reliable retrieval [#76](https://github.com/datastax/cass-operator/commit/3504367a5ac60f04724922e59cf86490ffb7e83d)
* Update to Operator SDK v0.17.0 [#78](https://github.com/datastax/cass-operator/commit/ac882984b78d9eb9e6b624ba0cfc11697ddfb3d2)
* Update Cassandra images to include metric-collector-for-apache-cassandra (MCAC) [#81](https://github.com/datastax/cass-operator/commit/4196f1173e73571985789e087971677f28c88d09)
* Run data cleanup after scaling up a datacenter [#80](https://github.com/datastax/cass-operator/commit/4a71be42b64c3c3bc211127d2cc80af3b69aa8e5)
* Requeue after the last node has its node-state label set to Started during cluster creation [#77](https://github.com/datastax/cass-operator/commit/b96bfd77775b5ba909bd9172834b4a56ef15c319)
* Remove delete verb from validating webhook [#75](https://github.com/datastax/cass-operator/commit/5ae9bee52608be3d2cd915e042b2424453ac531e)
* Add conditions to CassandraDatacenter status [#50](https://github.com/datastax/cass-operator/commit/8d77647ec7bfaddec7e0e616d47b1e2edb5a0495)
* Better support and safeguards for adding racks to a datacenter [#59](https://github.com/datastax/cass-operator/commit/0bffa2e8d084ac675e3e4e69da58c7546a285596)

## v1.1.0
* #27 Added a helm chart to ease installing.
* #23 #37 #46 Added a validating webhook for CassandraDatacenter.
* #43 Emit more events when reconciling a CassandraDatacenter.
* #47 Support `nodeSelector` to pin database pods to labelled k8s worker nodes.
* #22 Refactor towards less code listing pods.
* Several integration tests added.

## v1.0.0
* Project renamed to `cass-operator`.
* KO-281 Node replace added.
* KO-310 The operator will work to revive nodes that fail readiness for over 10 minutes
  by deleting pods.
* KO-317 Rolling restart added.
* K0-83 Stop the cluster more gracefully.
* KO-329 API version bump to v1beta1.

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
