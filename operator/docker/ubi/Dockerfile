ARG BASE_OS
FROM datastax/cass-operator:latest AS base

#############################################################

FROM ${BASE_OS} AS builder

# Update the builder layer and create user
RUN microdnf update && rm -rf /var/cache/yum && \
    microdnf install shadow-utils && microdnf clean all && \
    useradd -r -s /bin/false -U -G root cassandra

#############################################################
FROM ${BASE_OS}

ARG BASE_OS
ARG VERSION_STAMP=DEV

LABEL maintainer="DataStax, Inc <info@datastax.com>"
LABEL name="cass-operator"
LABEL vendor="DataStax, Inc"
LABEL release="${VERSION_STAMP}"
LABEL summary="DataStax Kubernetes Operator for Apache Cassandra "
LABEL description="The DataStax Kubernetes Operator for Apache Cassandra®. This operator handles the provisioning and day to day management of Apache Cassandra based clusters. Features include configuration deployment, node remediation, and automatic upgrades."

# Update the builder layer and create user
RUN microdnf update && rm -rf /var/cache/yum && \
    microdnf install procps-ng && microdnf clean all

# Copy user accounts information
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/shadow /etc/shadow
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/gshadow /etc/gshadow

# Copy operator binary
COPY --from=base /go/bin/operator /operator
COPY ./operator/docker/ubi/LICENSE /licenses/

RUN mkdir -p /var/lib/cass-operator/
RUN echo ${BASE_OS} > /var/lib/cass-operator/base_os

RUN chown cassandra:root /operator && \
    chmod 0555 /operator

USER cassandra:root


ENTRYPOINT ["/operator"]