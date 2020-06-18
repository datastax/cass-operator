# Sample SSL Certificates and Keys

## Requirements

* [CFSSL](https://cfssl.org/)

## Generate Certificate Authority

```bash
# Create CA from a JSON template
cfssl gencert -initca ca.csr.json | cfssljson -bare ca

# Create the secret resource
kubectl create secret tls ca-cert --cert ca.pem --key ca-key.pem
```

## Generate Ingress Certificate

```bash
# Retrieve the host IDs to include in the Ingress CSR
# Make sure all nodes are up, then extract the values from
# the nodeStatuses field.
kubectl get cassdc sample-dc -o yaml

# Create and sign Ingress certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem ingress.csr.json | cfssljson -bare ingress

# Create the secret resource
kubectl create secret tls sample-cluster-sample-dc-cert --cert ingress.pem --key ingress-key.pem
```

## Generate Client Certificate

```bash
# Create and sign Client certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem client.csr.json | cfssljson -bare client
```
