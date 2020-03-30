#!/bin/bash

set -euf -o pipefail
set -x

bundle="$(mktemp)"

echo '---' >> "$bundle"
cat operator/deploy/namespace.yaml >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/service_account.yaml >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/role.yaml >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/role_binding.yaml >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/operator.yaml >> "$bundle"

grep -v x-kubernetes-preserve-unknown-fields < "$bundle" > docs/user/cass-operator-manifests-pre-1.15.yaml
mv "$bundle" docs/user/cass-operator-manifests.yaml
