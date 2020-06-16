#!/bin/bash

set -euf -o pipefail
set -x

echo its up to the user to delete the kind clusters before and after ';)'
echo kind delete cluster --name k-1-13
echo kind delete cluster --name k-1-14
echo kind delete cluster --name k-1-15
echo kind delete cluster --name k-1-16
echo kind delete cluster --name k-1-17

kind create cluster --image kindest/node:v1.13.12 --name k-1-13
hack/release/make-yaml-bundle.sh

kind create cluster --image kindest/node:v1.14.10 --name k-1-14
hack/release/make-yaml-bundle.sh

kind create cluster --image kindest/node:v1.15.11 --name k-1-15
hack/release/make-yaml-bundle.sh

kind create cluster --image kindest/node:v1.16.9 --name k-1-16
hack/release/make-yaml-bundle.sh

kind create cluster --image kindest/node:v1.17.5 --name k-1-17
hack/release/make-yaml-bundle.sh
