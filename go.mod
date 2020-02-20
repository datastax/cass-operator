module github.com/riptano/dse-operator

go 1.13

require (
	github.com/magefile/mage v1.9.0
	github.com/onsi/ginkgo v1.10.1
	github.com/onsi/gomega v1.7.0
	github.com/riptano/dse-operator/mage v0.0.0-00010101000000-000000000000
)

replace github.com/riptano/dse-operator/mage => ./mage

replace github.com/riptano/dse-operator/operator => ./operator
