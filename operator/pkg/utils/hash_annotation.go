// Copyright DataStax, Inc.
// Please see the included license file for details.

package utils

import (
	"crypto/sha256"
	"encoding/base64"

	"k8s.io/kubernetes/pkg/util/hash"
)

type Annotated interface {
	GetAnnotations() map[string]string
	SetAnnotations(annotations map[string]string)
}

const resourceHashAnnotationKey = "cassandra.datastax.com/resource-hash"

func ResourcesHaveSameHash(r1, r2 Annotated) bool {
	a1 := r1.GetAnnotations()
	a2 := r2.GetAnnotations()
	if a1 == nil || a2 == nil {
		return false
	}
	return a1[resourceHashAnnotationKey] == a2[resourceHashAnnotationKey]
}

func AddHashAnnotation(r Annotated) {
	hash := deepHashString(r)
	m := r.GetAnnotations()
	if m == nil {
		m = map[string]string{}
	}
	m[resourceHashAnnotationKey] = hash
	r.SetAnnotations(m)
}

func deepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}
