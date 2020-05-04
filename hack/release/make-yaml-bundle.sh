#!/bin/bash

set -euf -o pipefail
set -x

# uses helm v3.1.2, kubectl v1.18.2, yq v3.2.1

bundle="$(mktemp)"

echo '---' >> "$bundle"
cat operator/deploy/namespace.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
helm template ./charts/cass-operator-chart/ -n cass-operator | kubectl create --dry-run=client -o yaml -n cass-operator -f - >> "$bundle"

grep -v "x-kubernetes-preserve-unknown-fields\|matchPolicy" < "$bundle" > docs/user/cass-operator-manifests-pre-1.15.yaml
mv "$bundle" docs/user/cass-operator-manifests.yaml
