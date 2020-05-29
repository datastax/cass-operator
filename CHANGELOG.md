# Changelog

## v1.2.0
* Support for several k8s versions in the helm chart - [#97](https://github.com/datastax/cass-operator/commit/9d76ad8258aa4e1d4893a357546de7de80aef0a0)
* Ability to roll back a broken upgrade / configuration change - [#85](https://github.com/datastax/cass-operator/commit/86b869df65f8180524dc12ff11502f6f6889eef5)
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
