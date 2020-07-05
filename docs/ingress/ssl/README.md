# Sample SSL Certificates and Keys

## Requirements

* [CFSSL](https://cfssl.org/)

## Generate Certificate Authority

```bash
# Create CA from a JSON template
cfssl gencert -initca ca.csr.json | cfssljson -bare ca

# Create the secret resource
kubectl create secret generic ca-cert --from-file=tls.cert=ca.pem --from-file=tls.ca=ca.pem

# Create a truststore for the application
keytool -import -v -trustcacerts -alias CARoot -file ca.pem -keystore client.truststore
```

## Generate Ingress Certificate

```bash
# If you are using SNI, retrieve the host IDs to include in the Ingress CSR
# Make sure all nodes are up, then extract the values from the nodeStatuses field.
kubectl get cassdc sample-dc -o yaml

# Create and sign the Ingress certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem ingress.csr.json | cfssljson -bare ingress

# Create the secret resource
kubectl create secret generic sample-cluster-sample-dc-cert --from-file=tls.crt=ingress.pem --from-file=tls.key=ingress-key.pem --from-file=tls.ca=ca.pem
```

## Generate Client Certificate

```bash
# Create and sign Client certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem client.csr.json | cfssljson -bare client

# Create the keystore for the client
openssl pkcs12 -export -in client.pem -inkey client-key.pem -out client.p12
keytool -importkeystore -destkeystore client.keystore -srckeystore client.p12 -srcstoretype PKCS12
```
