##########
# NOTE: When building this image, there is an assumption that you are in the top level directory of the repository.
# $ docker build . -f operator/Dockerfile -t operator
##########
# "builder_base" compiles and tests the code
FROM golang:1.12-stretch as builder_base

# Disable cgo - this makes static binaries that will work on an Alpine image
ENV CGO_ENABLED=0

# Target os and arch
ENV GOOS linux
ENV GOARCH amd64

# Copy in source code
WORKDIR /dse-operator/operator

# Grab the dependencies
COPY ./operator/go.mod ./operator/go.sum ./
RUN go mod download

# Copy in source
COPY operator ./

# Run the tests
RUN go test -short -v ./...

# Build any code not touched by tests (the generated client)
RUN go build -v ./...

# Install package
RUN go build -v -o /go/bin/dse-operator-linux-amd64 cmd/manager/main.go

# Second stage
# "builder" copies in the code for a smaller image
FROM builder_base AS builder

FROM alpine:3.9

ENV GOPATH=/root/go
WORKDIR /root/go

# All we need from the builder image is operator executable
COPY --from=builder /go/bin/dse-operator-linux-amd64 bin/operator

CMD [ "/root/go/bin/operator" ]
