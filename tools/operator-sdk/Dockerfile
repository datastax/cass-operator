##############################################
#
# Stage 1: Installing the operator-sdk
#
##############################################

FROM golang:1.13-stretch as install-operator-sdk

# This Dockerfile sets up a clean go 1.13 environment with operator-sdk installed and tested.
#
# It is not part of any operator build system, but rather exists to serve as reference clean
# install of operator-sdk (pinned to a released version), their example project, and the kubernetes
# CLI tools, including the client-gen code generator.
#
# A good way for a developer to use it would be to mount the cass-operator git repo into the GOPATH
# inside the container. Something like...
# docker run -v /Users/jim/datastax/cass-operator:/go/src/github.com/datastax/cass-operator ...

ENV OP_SDK_VERSION=v0.17.0
ENV GOPROXY=https://proxy.golang.org,https://gocenter.io,direct

WORKDIR /usr/bin

# Download the specific operator-sdk we are pinned to
# and assign permissions
RUN curl -OJL https://github.com/operator-framework/operator-sdk/releases/download/${OP_SDK_VERSION}/operator-sdk-${OP_SDK_VERSION}-x86_64-linux-gnu
RUN mv operator-sdk-${OP_SDK_VERSION}-x86_64-linux-gnu operator-sdk
RUN chmod 700 operator-sdk


##############################################
#
# Stage 2: Testing the operator-sdk
#
##############################################

FROM install-operator-sdk as test-operator-sdk

# This stage exists to test the operator-sdk itself using their example project,
# and the kubernetes CLI tools, including the client-go code generator.

# This git configuration is required to run "operator-sdk new"
RUN git config --global user.email "you@example.com"
RUN git config --global user.name "Your Name"

# The next steps follow the operator-sdk getting started tutorial
# located at: https://github.com/operator-framework/operator-sdk#create-and-deploy-an-app-operator
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
    echo '        _ "sigs.k8s.io/controller-tools/pkg/crd"'; \
    echo ')'; \
    } > $GOPATH/src/github.com/example-inc/memcached-operator/tools.go

RUN GO111MODULE=on go mod tidy
RUN GO111MODULE=on go mod download
RUN GO111MODULE=on operator-sdk generate k8s --verbose
RUN GO111MODULE=on operator-sdk generate crds --verbose
RUN GO111MODULE=on operator-sdk generate csv --verbose

# The next steps use the kubernetes code-generator to make a client
# The code-generator currently needs to exist in the
# pre-module style path of $GOPATH/src/...
# So we copy the specific version that this operator-sdk pulled down and move it
# into the required path.
WORKDIR $GOPATH/src/k8s.io
RUN cp -r "/go/pkg/mod/k8s.io/code-generator@v0.17.4" "code-generator"

WORKDIR $GOPATH/src/k8s.io/code-generator
RUN chmod 700 generate-groups.sh

WORKDIR $GOPATH/src/github.com/example-inc/memcached-operator

RUN sed -i 's|.*type Memcached struct.*|// +genclient\n&|' pkg/apis/memcached/v1alpha1/memcached_types.go

RUN cd $GOPATH/src/k8s.io/code-generator

RUN GO111MODULE=on $GOPATH/src/k8s.io/code-generator/generate-groups.sh \
    client,lister,informer \
    github.com/example-inc/memcached-operator/pkg/generated \
    github.com/example-inc/memcached-operator/pkg/apis \
    memcached:v1alpha1

# This fails right now :(
# RUN GO111MODULE=on go build ./...

RUN GO111MODULE=on go build ./cmd/manager
