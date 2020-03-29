// Copyright DataStax, Inc.
// Please see the included license file for details.

// This file exists to indicate to "go mod tidy" that the following
// libraries are needed, even though those libraries are not directly
// used by the project.
//
// This causes "go mod vendor" to fetch these libraries.
// Also the operator-sdk tooling requires them for code generation.

// +build tools

package tools

import (
	// Code generators built at runtime.
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/conversion-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"
	_ "k8s.io/gengo/args"
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
	_ "sigs.k8s.io/controller-tools/pkg/crd"
)
