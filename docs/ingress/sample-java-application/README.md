# Sample Application with Kubernetes Ingress

This project is to illustrates how to configure and validate connectivity to Cassandra clusters running within Kubernetes. There are three reference client implementations available:

* mTLS and SNI based balancing
* Load balancing with mTLS
* Simple load balancing

At this time there is some _slight_ tweaking required to the configuration files to specify the keystore, truststore, and approach to use.

Any connections requiring TLS support should place their keystore an truststore in the `src/main/resources/` directory. If you followed the [SSL](../ssl) guide then you should already have these files available.

## Building and Running

```
mvn exec:exec@ingress
Discovered Nodes
sample-dc:sample-rack:fd280adc-e55e-4f3d-97d1-138a1e1abef4
sample-dc:sample-rack:fd280adc-e55e-4f3d-97d1-138a1e1abef4
sample-dc:sample-rack:fd280adc-e55e-4f3d-97d1-138a1e1abef4

Coordinator: sample-dc:sample-rack:fd280adc-e55e-4f3d-97d1-138a1e1abef4
[data_center:'sample-dc', rack:'sample-rack', host_id:a7a45d6e-70e3-4e6d-b29c-5dba9a61a282, release_version:'3.11.6']

Coordinator: sample-dc:sample-rack:fd280adc-e55e-4f3d-97d1-138a1e1abef4
[data_center:'sample-dc', rack:'sample-rack', host_id:7e2921a6-e170-4a4f-bf0f-011ab83b3739, release_version:'3.11.6']
[data_center:'sample-dc', rack:'sample-rack', host_id:fd280adc-e55e-4f3d-97d1-138a1e1abef4, release_version:'3.11.6']
```
