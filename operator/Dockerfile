##########
# NOTE: When building this image, there is an assumption that you are in the top level directory of the repository.
# $ docker build . -f operator/Dockerfile -t operator
##########
# "builder" compiles and tests the code
# This stage name is important to the Cloud Platform CI/CD infrastructure, and should be preserved
FROM golang:1.13-stretch as builder

# Disable cgo - this makes static binaries that will work on an Alpine image
ENV CGO_ENABLED=0
ENV GOPROXY=https://proxy.golang.org,https://gocenter.io,direct

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
# Produce a smaller image than the one used to build the code
FROM alpine:3.9

ENV GOPATH=/root/go
WORKDIR /root/go

# All we need from the builder image is operator executable
COPY --from=builder /go/bin/dse-operator-linux-amd64 bin/operator

CMD [ "/root/go/bin/operator" ]
