module github.com/riptano/dse-operator/mage

go 1.13

require (
	github.com/magefile/mage v1.9.0
	github.com/otiai10/copy v1.0.2
	github.com/pkg/errors v0.8.1 // indirect
	github.com/stretchr/testify v1.4.0
	gopkg.in/yaml.v2 v2.2.5
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.0.0-20191121015604-11707872ac1c // indirect
	k8s.io/apimachinery v0.0.0-20191123233150-4c4803ed55e3 // indirect
)

replace github.com/riptano/dse-operator/mage => ./
