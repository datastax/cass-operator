# Sample SSL Certificates and Keys

## Requirements

* [CFSSL](https://cfssl.org/)

## Generate Certificate Authority

```bash
# Create CA from a JSON template
cfssl gencert -initca ca.csr.json | cfssljson -bare ca

# Generate Base64 values for the generated certificate's
# secret
base64 -w 0 ca.pem
```

## Generate Ingress Certificate

```bash
# Retrieve the host IDs to include in the Ingress CSR
# Make sure all nodes are up, then extract the values from
# the nodeStatuses field.
kubectl get cassdc sample-dc -o yaml

# Create and sign Ingress certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem ingress.csr.json | cfssljson -bare ingress

# Generate Base64 values for the generated certificate's
# secret
base64 -w 0 ingress-key.pem
base64 -w 0 ingress.pem
```

## Generate Client Certificate

```bash
# Create and sign Client certificate
cfssl gencert -ca ca.pem -ca-key ca-key.pem client.csr.json | cfssljson -bare client
```
