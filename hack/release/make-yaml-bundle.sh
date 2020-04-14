#!/bin/bash

set -euf -o pipefail
set -x

bundle="$(mktemp)"

echo '---' >> "$bundle"
cat operator/deploy/namespace.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/service_account.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/role.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/role_binding.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
cat operator/deploy/crds/cassandra.datastax.com_cassandradatacenters_crd.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
yq w --doc 3 operator/deploy/operator.yaml 'spec.template.spec.containers[0].image' 'datastax/cass-operator:1.0.0' >> "$bundle"

grep -v x-kubernetes-preserve-unknown-fields < "$bundle" > docs/user/cass-operator-manifests-pre-1.15.yaml
mv "$bundle" docs/user/cass-operator-manifests.yaml

yq w -d'*' -i docs/user/cass-operator-manifests.yaml metadata.namespace 'cass-operator'
yq w -d7 -i docs/user/cass-operator-manifests.yaml 'webhooks[*].clientConfig.service.namespace' 'cass-operator'
#
yq w -d'*' -i docs/user/cass-operator-manifests-pre-1.15.yaml metadata.namespace 'cass-operator'
yq w -d7 -i docs/user/cass-operator-manifests-pre-1.15.yaml 'webhooks[*].clientConfig.service.namespace' 'cass-operator'
