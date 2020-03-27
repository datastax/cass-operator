#!/bin/bash

set -euf -o pipefail

echo '---' > bundle.yaml
cat operator/deploy/service_account.yaml >> bundle.yaml

echo '---' >> bundle.yaml
cat operator/deploy/role.yaml >> bundle.yaml

echo '---' >> bundle.yaml
cat operator/deploy/role_binding.yaml >> bundle.yaml

echo '---' >> bundle.yaml
cat operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml >> bundle.yaml

echo '---' >> bundle.yaml
cat operator/deploy/operator.yaml >> bundle.yaml

grep -v x-kubernetes-preserve-unknown-fields < bundle.yaml > bundle-k8s-1.13-1.14.yaml
