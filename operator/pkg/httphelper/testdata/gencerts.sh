#!/usr/bin/env bash

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
CERTS_DIR="$SCRIPT_DIR"

function openssl() {
    # Ensure the certificates directory exists
    mkdir -p "$CERTS_DIR"

    # This just runs the openssl command but in a container with all the
    # extensions and whatnot we need.
    docker run -i -v "$CERTS_DIR:/export" -w /export frapsoft/openssl "$@"
}

function create_certificates() {
    openssl req -extensions v3_ca -new -x509 -days 36500 -nodes \
        -subj "/CN=MyRootCA" -newkey rsa:2048 -sha512 -out ca.crt \
        -keyout ca.key \
        || return $?

    openssl req -extensions v3_ca -new -x509 -days 36500 -nodes \
        -subj "/CN=MyEvilCA" -newkey rsa:2048 -sha512 -out evil_ca.crt \
        -keyout evil_ca.key \
        || return $?

    openssl req -new -keyout server.key -nodes -newkey rsa:2048 \
        -subj "/CN=localhost" \
        | openssl x509 -req -CAkey ca.key -CA ca.crt -days 36500 \
        -set_serial $RANDOM -sha512 -out server.crt \
        || return $?

    openssl req -new -keyout client.key -nodes -newkey rsa:2048 \
        -subj "/CN=SomeFancyPantsClient" \
        | openssl x509 -req -CAkey ca.key -CA ca.crt -days 36500 \
        -set_serial $RANDOM -sha512 -out client.crt \
        || return $?

    openssl rsa -in server.key -out server.rsa.key
    
    openssl pkcs8 -topk8 -in server.key -out server.encrypted.key \
        -passout pass:bob
}

create_certificates
