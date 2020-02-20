package hello_world_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/riptano/dse-operator/mage/kubectl"
)

var _ = Describe("Simple secret test", func() {
	Context("when in a new cluster", func() {
		secretName := "test-secret"

		BeforeSuite(func() {
			// Create our secret
			err := kubectl.CreateSecretLiteral(secretName, "usr", "psw").ExecV()
			Expect(err).ToNot(HaveOccurred())
		})

		AfterSuite(func() {
			// Clean up our secret
			err := kubectl.DeleteByTypeAndName("secret", secretName).ExecV()
			Expect(err).ToNot(HaveOccurred())
		})

		It("can retrieve a specific secret", func() {
			_, err := kubectl.GetByTypeAndName("secret", secretName).FormatOutput("json").Output()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
