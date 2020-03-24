#!/bin/bash
# For the curl invocations, see https://docs.openshift.com/dedicated/3/rest_api/apis-certificates.k8s.io/v1beta1.CertificateSigningRequest.html

CERT_NAME="cassandradatacenter-webhook-service.default"

TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)

ENDPOINT=$KUBERNETES_SERVICE_HOST:$KUBERNETES_PORT_443_TCP_PORT

# curl -X POST -H ‘Content-Type: application/yaml’-data-binary @csr.yaml -cacert ca.crt -H "Authorization: Bearer $TOKEN" ‘https://<cluster-ip>/apis/batch/v1/namespaces/default/jobs’

echo "Creating CSR\n"

# curl -X POST -sSk \
curl -s -k \
    -X POST \
    --data-binary @/csr.yaml \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    -H 'Accept: application/json' \
    https://$ENDPOINT/apis/certificates.k8s.io/v1beta1/certificatesigningrequests > pending-csr.yaml 

sleep 3

echo "Approving CSR\n"

# We must add a condition to make the approval work
sed 's/"status": {/"status": {"conditions":[{"type":"Approved","reason":"KubectlApprove","message":"This CSR was approved by cass-operator-initcontainer."}]/' pending-csr.yaml > pending-csr-2.yaml

curl -k \
    -X PUT \
    --data-binary @pending-csr-2.yaml \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Content-Type: application/yaml' \
    -H 'Accept: application/json' \
    https://$ENDPOINT/apis/certificates.k8s.io/v1beta1/certificatesigningrequests/$CERT_NAME/approval

sleep 3

echo "Storing CSR\n"

curl  -k \
    -H "Authorization: Bearer $TOKEN" \
    -H 'Accept: application/json' \
    https://$ENDPOINT/apis/certificates.k8s.io/v1beta1/certificatesigningrequests/$CERT_NAME | jq ".status.certificate" > approved-certificate.json

# We must remove the quotes around the certificate data
sed 's/"//g' approved-certificate.json > raw-certificate.json

cat raw-certificate.json | openssl base64 -d -A -out /tmp/k8s-webhook-server/serving-certs/tls.crt

# Copy the tls.key to the mounted volume

cp /tls.key /tmp/k8s-webhook-server/serving-certs/tls.key

cp /run/secrets/kubernetes.io/serviceaccount/ca.crt /tmp/k8s-webhook-server/serving-certs/ca.crt

# Get the ca bundle

echo "\nCA_BUNDLE\n"

# Note: we must base64 encode the ca.crt and then remove newlines
CA_BUNDLE=$(cat /run/secrets/kubernetes.io/serviceaccount/ca.crt | base64 | tr -d '\n')

echo "CA_BUNDLE after base64 encoding and removing newlines\n"

printf "%s" "$CA_BUNDLE"

WEBHOOK_TEMPLATE=$(cat /webhook_template.yaml)

echo ""

echo "WEBHOOK_TEMPLATE"

printf "%s" "$WEBHOOK_TEMPLATE"

WEBHOOK_YAML=${WEBHOOK_TEMPLATE/AUTOMATICALLY_REPLACED_CA_BUNDLE/$CA_BUNDLE}

echo ""
echo "webhook.yaml"
echo ""

printf "%s" "$WEBHOOK_YAML"

echo ""

printf "%s" "$WEBHOOK_YAML" > /webhook.yaml

cp /webhook.yaml /tmp/k8s-webhook-server/serving-certs/webhook.yaml

echo "Created webhook.yaml"

curl -k \
    -X POST \
    --data-binary @/webhook.yaml \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    -H 'Accept: application/json' \
    https://$ENDPOINT/apis/admissionregistration.k8s.io/v1beta1/validatingwebhookconfigurations

echo "Created ValidatingWebhookConfiguration"

curl -k \
    -X POST \
    --data-binary @/webhook_service.yaml \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/yaml" \
    -H 'Accept: application/json' \
    https://$ENDPOINT/api/v1/namespaces/default/services

echo "Created webhook service"
