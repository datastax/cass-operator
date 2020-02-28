package kubectl

import (
	"fmt"
	"os"
	"time"

	shutil "github.com/riptano/dse-operator/mage/sh"
)

func WatchPods() {
	shutil.RunVPanic("watch", "-n1", "kubectl", "get", "pods")
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
	args := []string{k.Command}
	for _, r := range k.Args {
		args = append(args, r)
	}
	for k, v := range k.Flags {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	return args
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

func WaitForOutput(k KCmd, expected string, seconds int) error {
	c := make(chan string)
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	cquit := make(chan bool)
	defer close(cquit)

	var actual string
	var err error

	go func() {
		for actual != expected {
			select {
			case <-cquit:
				return
			default:
				actual, err = k.Output()
				// Execute at most once every second
				time.Sleep(time.Second)
			}
		}
		c <- actual
	}()

	select {
	case <-timer.C:
		msg := fmt.Sprintf("Timed out waiting for value. Expected %s, but got %s.", expected, actual)
		if err != nil {
			msg = fmt.Sprintf("%s\nThe following error occured while querying k8s: %v", msg, err)
		}
		e := fmt.Errorf(msg)
		return e
	case <-c:
		return nil
	}
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
