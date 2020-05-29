# Changelog

## v1.2.0
* Better support for several k8s versions - [#97](https://github.com/datastax/cass-operator/commit/9d76ad8258aa4e1d4893a357546de7de80aef0a0)
* Fix broken upgrade - https://github.com/datastax/cass-operator/commit/86b869df65f8180524dc12ff11502f6f6889eef5
* Create default kubeconfig when setting up k3d (#91) 1845cab4e2ada881b01d84331ccb20b1d4742b44
* Update helm crd and test generate targets in gh workflow (#93) ac23560f156202a9a90e198776da980a77455dd6
* Mount root as read-only and temp dir as memory emptyvol #86 (#89) 0474057e8339da4f89b2e901ab697f10a2184d78
* ko-364 fix managed by (#84) 39519b8bae8795542a5fb16a844aeb55cf3b2737
* Add diagrams (#90) f1fe5fb3e07cec71a2ba0df8fabfec2b7751a95b
* Add PodTemplateSpec in cassdc crd spec (#67) 7ce9077beab7fb38f0796c303c9e3a3610d94691
* Merge k3d config with default/current kubeconfig (#87) 654fb674a40dd878c7ccdab6e635999200e45851
* Update gitignore (#88) a7482f2cdb943d184ef9a65d6897241944f1e16a
* Support k3d + abstract common targets from all cluster types (#79) c360cfce60888e54b10fdb3aaaa2e9521f6790cf
* Add logging of all events for more reliable retrieval (#76) 3504367a5ac60f04724922e59cf86490ffb7e83d
* Operator SDK v0.17.0 (#78) ac882984b78d9eb9e6b624ba0cfc11697ddfb3d2
* Update C* images to include metric-collector (#81) 4196f1173e73571985789e087971677f28c88d09
* KO-382 cleanup after scaleup (#80) 4a71be42b64c3c3bc211127d2cc80af3b69aa8e5
* requeue after the last node has its node-state label set to Started during cluster creation (#77) b96bfd77775b5ba909bd9172834b4a56ef15c319
* remove delete verb from validating webhook (#75) 5ae9bee52608be3d2cd915e042b2424453ac531e
* add --all flag to uninstall step (#73) c9d8520755d5c3cfa7c43035400fdc0c3759390b
* Add conditions (#50) 8d77647ec7bfaddec7e0e616d47b1e2edb5a0495
* fix up logic that cleans up test namespaces when ensuring an empty cluster (#68) 82e51f489a997953e7b198f2525fab681de4f0d2
* KO-397  single go.mod in root (#62) 7a7c62bb5567b8ebe8b574decfb8ff39da4de224
* KO-396 remove operator init (#60) 408bd59bb5d20e1d8365e05cc154c557e49d34bd
* KO-398 push to GH packages instead of artifactory (#65) 8d8b2452572e440ffe5b7a20f978a203658125c8
* KO-219 KO-360 KO-361 Handle adding racks (#59) 0bffa2e8d084ac675e3e4e69da58c7546a285596
* Move to 1.1.1-snapshot (#58) 229855d387e8f7f87d6be6814a4eaaf2b4fd4c36

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
