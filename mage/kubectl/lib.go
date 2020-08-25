// Copyright DataStax, Inc.
// Please see the included license file for details.

package kubectl

import (
	"fmt"
	"os"
	"os/user"
	"regexp"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

func GetKubeconfig(createDefault bool) string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		defaultDir := fmt.Sprintf("%s/.kube/", usr.HomeDir)
		kubeconfig = fmt.Sprintf("%s/config", defaultDir)
		if _, err := os.Stat(kubeconfig); createDefault && os.IsNotExist(err) {
			os.MkdirAll(defaultDir, 0755)
			file, err := os.Create(kubeconfig)
			mageutil.PanicOnError(err)
			file.Close()
		}
	}
	return kubeconfig
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
	args = append(args, k.Args...)
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

func DeleteNamespace(namespace string) KCmd {
	args := []string{"namespace", namespace}
	return KCmd{Command: "delete", Args: args}
}

func CreateSecretLiteral(name string, user string, pw string) KCmd {
	args := []string{"secret", "generic", name}
	flags := map[string]string{
		"from-literal=username": user,
		"from-literal=password": pw,
	}
	return KCmd{Command: "create", Args: args, Flags: flags}
}

func Taint(node string, key string, value string, effect string) KCmd {
	args := []string{"nodes", node, fmt.Sprintf("%s=%s:%s", key, value, effect)}
	return KCmd{Command: "taint", Args: args}
}

func Annotate(resource string, name string, key string, value string) KCmd {
	args := []string{resource, name, fmt.Sprintf("%s=%s", key, value)}
	return KCmd{Command: "annotate", Args: args}
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

func erasePreviousLine() {
	//cursor up one line
	fmt.Print("\033[A")

	//erase line
	fmt.Print("\033[K")
}

func waitForOutputPattern(k KCmd, pattern string, seconds int) error {
	re := regexp.MustCompile(pattern)
	c := make(chan string)
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	cquit := make(chan bool)
	defer close(cquit)

	counter := 0
	outputIsTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	var actual string
	var err error

	go func() {
		printedRerunMsg := false
		for !re.MatchString(actual) {
			select {
			case <-cquit:
				fmt.Println("")
				return
			default:
				actual, err = k.Output()
				if outputIsTerminal && counter > 0 {
					erasePreviousLine()

					if printedRerunMsg {
						// We need to erase both the new exec output,
						// as well as our previous "rerunning" line now
						erasePreviousLine()
					}

					fmt.Printf("Rerunning previous command (%v)\n", counter)
					printedRerunMsg = true
				}
				counter++
				// Execute at most once every two seconds
				time.Sleep(time.Second * 2)
			}
		}
		c <- actual
	}()

	select {
	case <-timer.C:
		var expectedPhrase string
		expectedPhrase = "Expected output to match regex: "
		msg := fmt.Sprintf("Timed out waiting for value. %s '%s', but '%s' did not match", expectedPhrase, pattern, actual)
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
	return waitForOutputPattern(k, pattern, seconds)
}

func WaitForOutput(k KCmd, expected string, seconds int) error {
	return waitForOutputPattern(k, fmt.Sprintf("^%s$", regexp.QuoteMeta(expected)), seconds)
}

func WaitForOutputContains(k KCmd, expected string, seconds int) error {
	return waitForOutputPattern(k, regexp.QuoteMeta(expected), seconds)
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

func GetNodeNameForPod(podName string) KCmd {
	json := "jsonpath={.spec.nodeName}"
	return Get(fmt.Sprintf("pod/%s", podName)).FormatOutput(json)
}
