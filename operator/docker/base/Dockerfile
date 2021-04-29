##########
# NOTE: When building this image, there is an assumption that you are in the top level directory of the repository.
# $ docker build . -f operator/Dockerfile -t operator
##########

# This arg is only used when building UBI images; however, if it does not have 
# a value, the build will fail even if not using a UBI target. To get around this
# we give the argument a dummy value so that an arg need not be provided, but it
# will still fail if building a UBI image without providing an explicit value.
# The value is prefixed with 'datastax/' to ensure it points to an image that we
# can ensure does not exist.
ARG BASE_OS=datastax/doesnotexist

# "builder" compiles and tests the code
# This stage name is important to the Cloud Platform CI/CD infrastructure, and should be preserved
FROM --platform=${BUILDPLATFORM} golang:1.14-stretch as builder

# Disable cgo - this makes static binaries that will work on an Alpine image
ENV CGO_ENABLED=0
ENV GOPROXY=https://proxy.golang.org,https://gocenter.io,direct

# Target os and arch
ARG TARGETARCH
ENV GOOS=linux
ENV GOARCH=${TARGETARCH}
ENV GOPATH=/go


WORKDIR /cass-operator

COPY go.mod go.sum ./

# Download go deps (including mage)
RUN go mod download

# At this point, we have the top level go.mod, the ./mage level go.mod,
# and the ./operator level go.mod without copying any of the source code yet.
# This means that the dependencies should be nicely without having
# to rerun every time we modify our lib code

# Install mage
# The new go module system will put the version number in the path, so we
# need to wildcard here to work with whatever version we are pinned to
RUN cd $GOPATH/pkg/mod/github.com/magefile/mage* && go run bootstrap.go

# Copy in source code
COPY magefile.go ./
COPY mage ./mage
COPY operator ./operator

ARG VERSION_STAMP=DEV

# Build any code not touched by tests (the generated client)
RUN MO_VERSION=${VERSION_STAMP} mage operator:buildGo

# Second stage
# Produce a smaller image than the one used to build the code
FROM alpine:3.9 as cass-operator
ENV GOPATH=/go

RUN mkdir -p /var/lib/cass-operator/
RUN touch /var/lib/cass-operator/base_os
WORKDIR /go

# All we need from the builder image is operator executable
ARG TARGETARCH
COPY --from=builder /cass-operator/build/bin/cass-operator-linux-${TARGETARCH} bin/operator

CMD [ "/go/bin/operator" ]

# UBI Image



#############################################################

FROM ${BASE_OS} AS builder-ubi

# Update the builder layer and create user
RUN microdnf update && rm -rf /var/cache/yum && \
    microdnf install shadow-utils && microdnf clean all && \
    useradd -r -s /bin/false -U -G root cassandra

#############################################################
FROM ${BASE_OS} AS cass-operator-ubi

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
COPY --from=builder-ubi /etc/passwd /etc/passwd
COPY --from=builder-ubi /etc/shadow /etc/shadow
COPY --from=builder-ubi /etc/group /etc/group
COPY --from=builder-ubi /etc/gshadow /etc/gshadow

# Copy operator binary
COPY --from=cass-operator /go/bin/operator /operator
COPY ./operator/docker/ubi/LICENSE /licenses/

RUN mkdir -p /var/lib/cass-operator/
RUN echo ${BASE_OS} > /var/lib/cass-operator/base_os

RUN chown cassandra:root /operator && \
    chmod 0555 /operator

USER cassandra:root

ENTRYPOINT ["/operator"]
