## The validating webhook.

The operator offers, and installs when possible, a validating webhook for
related CRDs. The webhook is intended to provide checks of the validity of an
update or create request, where there might be CRD-specific guardrails that are
not readily checked by implicit CRD validity. Such checks include preventing
renaming certain elements of the deployment, such as the the cassandra cluster
or the racks, which are core to the identity of a cassandra cluster.

Validating webhooks have specific requirements in kubernetes:
* They must be served over TLS
* The TLS service name where they are reached must match the subject of the certificate
* The CA signing the certificate must be either installed in the kube apiserver filesystem, or
explicitly configured in the kubernetes validatingwebhookconfiguration object.

The operator takes a progressive-enhancement approach to enabling this webhook,
which is described as follows:

The operator will look for, and if present, use, the certificates in the
default location that the controller-manager expects the certificates.  If the
files there don't exist, or the certificate does not appear to be valid, then
the operator will generate a self-signed CA, and attempt to update the various
kubernetes references to that certificate, specifically:
* The CA defined in the webhook
* The cert and key stored in the relevant secret in the cass-operator namespace.

If the cert and key are regenerated, then they will also be written to an
alternative location on disk, so that they can be consumed by the
controller-manager. Because the operator root filesystem is recommended to be
deployed read-only, and secret mount points are typically read-only as well, an
alternative location to host the certificate and key is chosen in a
memory-backed temporary kubernetes volume.

To avoid a prohibitive user experience, the webhook is configured to fail open.
This means that errors encountered in the above process will generate log
messages, but will not wholly prevent the operation of the cass-operator.
