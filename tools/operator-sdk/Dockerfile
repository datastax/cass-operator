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

ENV OP_SDK_VERSION=v0.9.0
ENV GOPROXY=https://gocenter.io

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
RUN GO111MODULE=on operator-sdk add api --api-version=memcached.com/v1alpha1 --kind=Memcached

# We now have to pull the code generator down as a separate step
# Add a tools.go to the example project that requires the code-generator
# see: https://github.com/docker-library/openjdk/blob/8a23a228bda7d2edeb4132fffd2d08c1e1fcf4ac/8-jdk/Dockerfile#L28-L36
RUN { \
    echo 'package tools'; \
    echo ''; \
    echo 'import ('; \
    echo '        _ "k8s.io/code-generator/cmd/client-gen"'; \
    echo '        _ "k8s.io/code-generator/cmd/conversion-gen"'; \
    echo '        _ "k8s.io/code-generator/cmd/deepcopy-gen"'; \
    echo '        _ "k8s.io/code-generator/cmd/informer-gen"'; \
    echo '        _ "k8s.io/code-generator/cmd/lister-gen"'; \
    echo '        _ "k8s.io/gengo/args"'; \
    echo '        _ "k8s.io/kube-openapi/cmd/openapi-gen"'; \
    echo '        _ "sigs.k8s.io/controller-tools/pkg/crd/generator"'; \
    echo ')'; \
    } > $GOPATH/src/github.com/example-inc/memcached-operator/tools.go

RUN GO111MODULE=on go mod vendor
RUN GO111MODULE=on operator-sdk generate k8s
RUN GO111MODULE=on operator-sdk generate openapi

# The next steps use the kubernetes code-generator to make a client

WORKDIR $GOPATH/src/k8s.io
RUN cp -r "/go/pkg/mod/k8s.io/code-generator@v0.0.0-20181203235156-f8cba74510f3" "code-generator"
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
