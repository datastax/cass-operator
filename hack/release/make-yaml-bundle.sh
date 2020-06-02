#!/bin/bash

set -euf -o pipefail
set -x

# uses helm v3.1.2, kubectl v1.18.2, yq v3.2.1

bundle="$(mktemp)"

k8sVer="$(kubectl version --short | grep Server | egrep -o 'v[0-9].[0-9]+')"

echo '---' >> "$bundle"
cat operator/deploy/namespace.yaml | yq r - >> "$bundle"

echo '---' >> "$bundle"
helm template ./charts/cass-operator-chart/ -n cass-operator --validate=true | kubectl create --dry-run=client -o yaml -n cass-operator -f - >> "$bundle"

mv "$bundle" docs/user/cass-operator-manifests-$k8sVer.yaml
