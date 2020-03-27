# We use the K8s code-generator to generate client boilerplate for our operator.
# More information can be found at: https://github.com/kubernetes/code-generator
FROM golang:1.13-stretch

ENV CGO_ENABLED=0
ENV GOPROXY=https://proxy.golang.org,https://gocenter.io,direct
ENV GOPATH=/go
ENV GO111MODULE=on

# Assume that when this image gets run, the operator code directory
# is mounted to /go/src/github.com/datastax/cass-operator/operator
ENV OPERATOR_DIR=/go/src/github.com/datastax/cass-operator/operator

# We download a version of the code-generator that matches the build-arg CODEGEN_VERSION.
# For regular use, we would use the version here that is specified in the operator's go.mod file
ARG CODEGEN_VERSION
ENV CODEGEN_BASEPATH=/go/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}

RUN go get -d k8s.io/code-generator@${CODEGEN_VERSION}

# Copy the downloaded code-generator directory into a new path
# that conforms to the old GOPATH style. The code-generator version
# we use does not currently support go module directory layouts
WORKDIR $GOPATH/src/k8s.io
RUN cp -r $CODEGEN_BASEPATH "code-generator"

# Install the various code-generators that we might use
WORKDIR $GOPATH/src/k8s.io/code-generator
RUN go install ./cmd/client-gen
RUN go install ./cmd/lister-gen
RUN go install ./cmd/informer-gen

# The code-generator wants to be executed while your working directory
# is set to your repo location, in the GOPATH style.
WORKDIR $GOPATH/src/github.com/datastax/cass-operator/operator


# We don't want newly generated files to be owned by root
# and we will most likely be running this image as another user.
# To make sure our user can run the code-generators, we have
# to make some permission changes
RUN chmod -R 777 /go
RUN mkdir -p /.cache/go-build
RUN chmod -R 777 /.cache


# To make running this as simple a possible, we mark the generate script
# as the entrypoint so that we only need to specify the args to it when running.
#
# For example, you could run this image with:
# docker run --rm -t \
#   -v /home/chris/go/src/github.com/datastax/cass-operator/operator:/go/src/github.com/datastax/cass-operator/operator operator-gen-client \
#   client \
#   github.com/datastax/cass-operator/operator/pkg/generated \
#   github.com/datastax/cass-operator/operator/pkg/apis \
#   cassandra:v1beta1
ENTRYPOINT ["/go/src/k8s.io/code-generator/generate-groups.sh"]
