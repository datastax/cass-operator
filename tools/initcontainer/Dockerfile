FROM alpine:3.9
RUN apk update
# The logic is implemented in bash
RUN apk add bash
# We use openssl for working with certs
RUN apk add openssl
RUN apk add curl
RUN apk add jq
# We use kubernetes for communication with the apiserver
#ADD https://storage.googleapis.com/kubernetes-release/release/v1.6.4/bin/linux/amd64/kubectl /usr/local/bin/kubectl
#RUN chmod 755 /usr/local/bin/kubectl
RUN mkdir -p /tmp/k8s-webhook-server/serving-certs
COPY ./tls.key /
COPY ./csr.yaml /
COPY ./webhook_template.yaml /
COPY ./webhook_service.yaml /
COPY ./setup-webhook.sh /bin
RUN chmod 755 /bin/setup-webhook.sh
CMD ["/bin/setup-webhook.sh"]
