package ginkgo_util

import (
	"fmt"
	"regexp"
	"time"

	ginkgo "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/riptano/dse-operator/mage/kubectl"
	mageutil "github.com/riptano/dse-operator/mage/util"
)

// Wrapper type to make it simpler to
// set a namespace one time and execute all of your
// KCmd objects inside of it, and then use Gomega
// assertions on panic
type NsWrapper struct {
	Namespace     string
	TestSuiteName string
	LogDir        string
	stepCounter   int
}

func NewWrapper(suiteName string, namespace string) NsWrapper {
	return NsWrapper{
		Namespace:     namespace,
		TestSuiteName: suiteName,
		LogDir:        genSuiteLogDir(suiteName),
		stepCounter:   1,
	}

}

func (k NsWrapper) ExecV(kcmd kubectl.KCmd) error {
	err := kcmd.InNamespace(k.Namespace).ExecV()
	return err
}

func (k NsWrapper) ExecVPanic(kcmd kubectl.KCmd) {
	err := kcmd.InNamespace(k.Namespace).ExecV()
	Expect(err).ToNot(HaveOccurred())
}

func (k NsWrapper) Output(kcmd kubectl.KCmd) (string, error) {
	out, err := kcmd.InNamespace(k.Namespace).Output()
	return out, err
}

func (k NsWrapper) OutputPanic(kcmd kubectl.KCmd) string {
	out, err := kcmd.InNamespace(k.Namespace).Output()
	Expect(err).ToNot(HaveOccurred())
	return out
}

func (k NsWrapper) WaitForOutput(kcmd kubectl.KCmd, expected string, seconds int) error {
	return kubectl.WaitForOutput(kcmd.InNamespace(k.Namespace), expected, seconds)
}

func (k NsWrapper) WaitForOutputPanic(kcmd kubectl.KCmd, expected string, seconds int) {
	err := kubectl.WaitForOutput(kcmd.InNamespace(k.Namespace), expected, seconds)
	Expect(err).ToNot(HaveOccurred())
}

func (k *NsWrapper) countStep() int {
	n := k.stepCounter
	k.stepCounter++
	return n
}

//===================================
// Logging functions for the NsWrapper
// that execute the Kcmd and then dump
// k8s logs for that namespace
//====================================
func sanitizeForLogDirs(s string) string {
	reg, err := regexp.Compile(`[\s\\\/\-\.,]`)
	mageutil.PanicOnError(err)
	return reg.ReplaceAllLiteralString(s, "_")
}

func genSuiteLogDir(suiteName string) string {
	datetime := time.Now().Format("2006.01.02_15:04:05")
	return fmt.Sprintf("../../build/kubectl_dump/%s/%s",
		sanitizeForLogDirs(suiteName), datetime)
}

func (ns *NsWrapper) genTestLogDir(description string) string {
	sanitizedDesc := sanitizeForLogDirs(description)
	return fmt.Sprintf("%s/%02d_%s", ns.LogDir, ns.countStep(), sanitizedDesc)
}

func (ns *NsWrapper) ExecAndLog(description string, kcmd kubectl.KCmd) {
	ginkgo.By(description)
	defer kubectl.DumpLogs(ns.genTestLogDir(description), ns.Namespace).ExecVPanic()
	execErr := ns.ExecV(kcmd)
	Expect(execErr).ToNot(HaveOccurred())
}

func (ns *NsWrapper) OutputAndLog(description string, kcmd kubectl.KCmd) string {
	ginkgo.By(description)
	defer kubectl.DumpLogs(ns.genTestLogDir(description), ns.Namespace).ExecVPanic()
	output, execErr := ns.Output(kcmd)
	Expect(execErr).ToNot(HaveOccurred())
	return output
}

func (ns *NsWrapper) WaitForOutputAndLog(description string, kcmd kubectl.KCmd, expected string, seconds int) {
	ginkgo.By(description)
	defer kubectl.DumpLogs(ns.genTestLogDir(description), ns.Namespace).ExecVPanic()
	execErr := ns.WaitForOutput(kcmd, expected, seconds)
	Expect(execErr).ToNot(HaveOccurred())
}
