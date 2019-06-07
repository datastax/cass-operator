##########
# NOTE: When building this image, there is an assumption that you are in the top level directory of the repository.
# $ docker build . -f operator/Dockerfile -t operator
##########
# "builder_base" compiles and tests the code
FROM openjdk:8-jdk-stretch AS builder_base


COPY operator operator/
COPY settings.gradle settings.gradle
COPY build.gradle build.gradle
COPY gradle gradle/
COPY gradlew gradlew

RUN ./gradlew build -x dockerImage --stacktrace --info # Note: this will also run goTest


# Second stage
# "builder" copies in the code for a smaller image
FROM builder_base AS builder

FROM alpine:3.9

ENV GOPATH=/root/go
WORKDIR /root/go

# All we need from the builder image is operator executable
COPY --from=builder /operator/build/bin/dse-operator-linux-amd64 bin/operator

CMD [ "/root/go/bin/operator" ]