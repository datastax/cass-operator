# How to test Red Hat

1. Build all basic containers
   - dse-server
   - cassandra
   - cass-operator
   - cass-config-builder
2. Push all to red hat
   - Wait for security scans
   - Fix issues if any
   - Publish, fixup tags
3. Build a bundle (referencing redhat registry for cass-operator)
4. Push the bundle to a _local_ registry
5. Build an index pointed at the _local_ registry
   - All versions, not just latest
6. Run OpenShift locally
7. Create cass-operator prereqs
8. Install a `CatalogSource` that references the local index (step 5)
9. Install a `Subscription` that reference the `CatalogSource` (step 8)
10. Check to make sure operator is up
11. Provision a CassandraDatacenter
12. Validate cassdc comes up
13. Check CQLSH, nodetool status, whatever
14. Tear down local openshift
14. Push bundle to Red Hat (step 4)
15. Validate tests pass
16. Publish the bundle
17. Run OpenShift locally
18. Install a `Subscription` that references the _PUBLIC_ openshift catalog
19. Check steps 10-13
20. Tear down local openshift
