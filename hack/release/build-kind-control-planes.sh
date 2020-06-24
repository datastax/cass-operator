#!/bin/bash

set -euf -o pipefail
set -x

kind create cluster --image kindest/node:v1.13.12 --name k-1-13
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.13.yaml
kind delete cluster --name k-1-13

kind create cluster --image kindest/node:v1.14.10 --name k-1-14
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.14.yaml
kind delete cluster --name k-1-14

kind create cluster --image kindest/node:v1.15.11 --name k-1-15
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.15.yaml
kind delete cluster --name k-1-15

kind create cluster --image kindest/node:v1.16.9 --name k-1-16
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.16.yaml
kind delete cluster --name k-1-16

kind create cluster --image kindest/node:v1.17.5 --name k-1-17
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.17.yaml
kind delete cluster --name k-1-17

kind create cluster --image kindest/node:v1.18.4 --name k-1-18
hack/release/make-yaml-bundle.sh
kubectl apply -f docs/user/cass-operator-manifests-v1.18.yaml
kind delete cluster --name k-1-18
