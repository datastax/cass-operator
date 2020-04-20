#!/bin/sh

set -euf -o pipefail
set -x

# uses diff-so-fancy from https://github.com/so-fancy/diff-so-fancy

opDeploy="operator/deploy"
chartTmpl="charts/cass-operator-chart/templates"
crdFilename="cassandra.datastax.com_cassandradatacenters_crd.yaml"

diff -u $opDeploy/role.yaml                   $chartTmpl/role.yaml | diff-so-fancy || true
diff -u $opDeploy/role_binding.yaml           $chartTmpl/rolebinding.yaml | diff-so-fancy || true
diff -u $opDeploy/cluster_role.yaml           $chartTmpl/clusterrole.yaml | diff-so-fancy || true
diff -u $opDeploy/cluster_role_binding.yaml   $chartTmpl/clusterrolebinding.yaml | diff-so-fancy || true
diff -u $opDeploy/service_account.yaml        $chartTmpl/serviceaccount.yaml | diff-so-fancy || true
diff -u $opDeploy/webhook_configuration.yaml  $chartTmpl/validatingwebhookconfiguration.yaml | diff-so-fancy || true
diff -u $opDeploy/operator.yaml               $chartTmpl/deployment.yaml | diff-so-fancy || true
diff -u $opDeploy/webhook_service.yaml        $chartTmpl/service.yaml | diff-so-fancy || true
diff -u $opDeploy/webhook_secret.yaml         $chartTmpl/secret.yaml | diff-so-fancy || true
diff -u $opDeploy/crds/$crdFilename           $chartTmpl/customresourcedefinition.yaml | diff-so-fancy || true
