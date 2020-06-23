#!/bin/bash

set -euf -o pipefail
set -x

kind create cluster --image kindest/node:v1.13.12 --name k-1-13
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-13

kind create cluster --image kindest/node:v1.14.10 --name k-1-14
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-14

kind create cluster --image kindest/node:v1.15.11 --name k-1-15
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-15

kind create cluster --image kindest/node:v1.16.9 --name k-1-16
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-16

kind create cluster --image kindest/node:v1.17.5 --name k-1-17
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-17

kind create cluster --image kindest/node:v1.18.4 --name k-1-18
hack/release/make-yaml-bundle.sh
kind delete cluster --name k-1-18
