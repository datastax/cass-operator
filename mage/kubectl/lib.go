// Copyright DataStax, Inc.
// Please see the included license file for details.

package kubectl

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"time"

	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

func GetKubeconfig() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	defaultConfig := fmt.Sprintf("%s/.kube/config", usr.HomeDir)
	return mageutil.EnvOrDefault("KUBECONFIG", defaultConfig)
}

func WatchPods() {
	shutil.RunVPanic("watch", "-n1", "kubectl", "get", "pods")
}

func WatchPodsInNs(namespace string) {
	shutil.RunVPanic("watch", "-n1", "kubectl", "get", "pods", fmt.Sprintf("--namespace=%s", namespace))
}

//==============================================
// KCmd represents an executable kubectl command
//==============================================
type KCmd struct {
	Command string
	Args    []string
	Flags   map[string]string
}

//==============================================
// Execute KCmd by running kubectl
//==============================================
func (k KCmd) ToCliArgs() []string {
	var args []string
	// Write out flags first because we don't know
	// if the command args will have a -- in them or not
	// and prevent our flags from working.
	for k, v := range k.Flags {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	args = append(args, k.Command)
	for _, r := range k.Args {
		args = append(args, r)
	}
	return args
}

func (k KCmd) ExecVCapture() (string, string, error) {
	return shutil.RunVCapture("kubectl", k.ToCliArgs()...)
}

func (k KCmd) ExecV() error {
	return shutil.RunV("kubectl", k.ToCliArgs()...)
}

func (k KCmd) ExecVPanic() {
	shutil.RunVPanic("kubectl", k.ToCliArgs()...)
}

func (k KCmd) Output() (string, error) {
	return shutil.Output("kubectl", k.ToCliArgs()...)
}

func (k KCmd) OutputPanic() string {
	return shutil.OutputPanic("kubectl", k.ToCliArgs()...)
}

//==============================================
// Helper functions to build up a KCmd object
// for common actions
//==============================================
func (k KCmd) InNamespace(namespace string) KCmd {
	return k.WithFlag("namespace", namespace)
}

func (k KCmd) FormatOutput(outputType string) KCmd {
	return k.WithFlag("output", outputType)
}

func (k KCmd) WithFlag(name string, value string) KCmd {
	if k.Flags == nil {
		k.Flags = make(map[string]string)
	}
	k.Flags[name] = value
	return k
}

func (k KCmd) WithLabel(label string) KCmd {
	k.Args = append(k.Args, "-l", label)
	return k
}

func ClusterInfoForContext(ctxt string) KCmd {
	args := []string{"--context", ctxt}
	return KCmd{Command: "cluster-info", Args: args}
}

func CreateNamespace(namespace string) KCmd {
	args := []string{"namespace", namespace}
	return KCmd{Command: "create", Args: args}
}

func CreateSecretLiteral(name string, user string, pw string) KCmd {
	args := []string{"secret", "generic", name}
	flags := map[string]string{
		"from-literal=username": user,
		"from-literal=password": pw,
	}
	return KCmd{Command: "create", Args: args, Flags: flags}
}

func CreateFromFiles(paths ...string) KCmd {
	var args []string
	for _, p := range paths {
		args = append(args, "-f", p)

	}
	return KCmd{Command: "create", Args: args}
}

func Get(args ...string) KCmd {
	return KCmd{Command: "get", Args: args}
}

func GetByTypeAndName(resourceType string, names ...string) KCmd {
	var args []string
	for _, n := range names {
		args = append(args, fmt.Sprintf("%s/%s", resourceType, n))
	}
	return KCmd{Command: "get", Args: args}
}

func GetByFiles(paths ...string) KCmd {
	var args []string
	for _, path := range paths {
		args = append(args, "-f", path)
	}
	return KCmd{Command: "get", Args: args}
}

func DeleteFromFiles(paths ...string) KCmd {
	var args []string
	for _, path := range paths {
		args = append(args, "-f", path)
	}
	return KCmd{Command: "delete", Args: args}
}

func Delete(args ...string) KCmd {
	return KCmd{Command: "delete", Args: args}
}

func DeleteByTypeAndName(resourceType string, names ...string) KCmd {
	var args []string
	for _, n := range names {
		args = append(args, fmt.Sprintf("%s/%s", resourceType, n))
	}
	return KCmd{Command: "delete", Args: args}
}

func ApplyFiles(paths ...string) KCmd {
	var args []string
	for _, path := range paths {
		args = append(args, "-f", path)
	}
	return KCmd{Command: "apply", Args: args}
}

func PatchMerge(resource string, data string) KCmd {
	args := []string{resource, "--patch", data, "--type", "merge"}
	return KCmd{Command: "patch", Args: args}
}

func PatchJson(resource string, data string) KCmd {
	args := []string{resource, "--patch", data, "--type", "json"}
	return KCmd{Command: "patch", Args: args}
}

func waitForOutputPattern(k KCmd, pattern string, expected string, seconds int) error {
	re := regexp.MustCompile(pattern)
	c := make(chan string)
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	cquit := make(chan bool)
	defer close(cquit)

	var actual string
	var err error

	go func() {
		for !re.MatchString(actual) {
			select {
			case <-cquit:
				return
			default:
				actual, err = k.Output()
				// Execute at most once every two seconds
				time.Sleep(time.Second * 2)
			}
		}
		c <- actual
	}()

	select {
	case <-timer.C:
		var expectedPhrase string
		expectedPhrase = "Expected to output to contain:"
		msg := fmt.Sprintf("Timed out waiting for value. %s %s, but got %s.", expectedPhrase, expected, actual)
		if err != nil {
			msg = fmt.Sprintf("%s\nThe following error occured while querying k8s: %v", msg, err)
		}
		e := fmt.Errorf(msg)
		return e
	case <-c:
		return nil
	}
}

func WaitForOutputPattern(k KCmd, pattern string, seconds int) error {
	return waitForOutputPattern(k, pattern, pattern, seconds)
}

func WaitForOutput(k KCmd, expected string, seconds int) error {
	return waitForOutputPattern(k, fmt.Sprintf("^%s$", regexp.QuoteMeta(expected)), expected, seconds)
}

func WaitForOutputContains(k KCmd, expected string, seconds int) error {
	return waitForOutputPattern(k, regexp.QuoteMeta(expected), expected, seconds)
}

func DumpAllLogs(path string) KCmd {
	//Make dir if doesn't exist
	_ = os.MkdirAll(path, os.ModePerm)
	args := []string{"dump", "-A"}
	flags := map[string]string{"output-directory": path}
	return KCmd{Command: "cluster-info", Args: args, Flags: flags}
}

func DumpLogs(path string, namespace string) KCmd {
	//Make dir if doesn't exist
	_ = os.MkdirAll(path, os.ModePerm)
	args := []string{"dump", "-n", namespace}
	flags := map[string]string{"output-directory": path}
	return KCmd{Command: "cluster-info", Args: args, Flags: flags}
}

func ExecOnPod(podName string, args ...string) KCmd {
	execArgs := []string{podName}
	execArgs = append(execArgs, args...)
	return KCmd{Command: "exec", Args: execArgs}
}
