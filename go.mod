module github.com/datastax/cass-operator

go 1.13

require (
	github.com/datastax/cass-operator/mage v0.0.0-00010101000000-000000000000
	github.com/magefile/mage v1.9.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	k8s.io/api v0.18.2
)

replace github.com/datastax/cass-operator/mage => ./mage

replace github.com/datastax/cass-operator/operator => ./operator
