FROM golang:1.12-stretch

# This Dockerfile sets up a clean go 1.12 environment with operator-sdk installed and tested.
#
# It is not part of any DSE operator build system, but rather exists to serve as reference clean
# install of operator-sdk (pinned to a released version), their example project, and the kubernetes
# CLI tools, including the client-gen code generator.
#
# A good way for a developer to use it would be to mount the dse-operator git repo into the GOPATH
# inside the container. Something like...
# docker run -v /Users/jim/datastax/dse-operator:/go/src/github.com/riptano/dse-operator ...

ENV OP_SDK_VERSION=v0.8.1

WORKDIR /usr/bin

RUN curl -OJL https://github.com/operator-framework/operator-sdk/releases/download/${OP_SDK_VERSION}/operator-sdk-${OP_SDK_VERSION}-x86_64-linux-gnu
RUN mv operator-sdk-${OP_SDK_VERSION}-x86_64-linux-gnu operator-sdk
RUN chmod 700 operator-sdk

# This git configuration is required to run "operator-sdk new"

RUN git config --global user.email "you@example.com"
RUN git config --global user.name "Your Name"

# The next steps follow the operator-sdk getting started tutorial

WORKDIR $GOPATH/src/github.com/example-inc/
RUN GO111MODULE=on operator-sdk new memcached-operator

WORKDIR $GOPATH/src/github.com/example-inc/memcached-operator
RUN operator-sdk add api --api-version=memcached.com/v1alpha1 --kind=Memcached
RUN operator-sdk generate k8s
RUN operator-sdk generate openapi

# The next steps use the kubernetes code-generator to make a client

WORKDIR $GOPATH/src/k8s.io
RUN cp -r "/go/pkg/mod/k8s.io/code-generator@v0.0.0-20181117043124-c2090bec4d9b" "code-generator"

WORKDIR $GOPATH/src/k8s.io/code-generator
RUN chmod 700 generate-groups.sh

WORKDIR $GOPATH/src/github.com/example-inc/memcached-operator

RUN sed -i 's|.*type Memcached struct.*|// +genclient\n&|' pkg/apis/memcached/v1alpha1/memcached_types.go

RUN GO111MODULE=on $GOPATH/src/k8s.io/code-generator/generate-groups.sh \
    client,lister,informer \
    github.com/example-inc/memcached-operator/pkg/generated \
    github.com/example-inc/memcached-operator/pkg/apis \
    memcached:v1alpha1

# This fails right now :(
# RUN GO111MODULE=on go build ./...

RUN GO111MODULE=on go build ./cmd/manager
