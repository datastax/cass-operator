// Copyright DataStax, Inc.
// Please see the included license file for details.

package dockerutil

import (
	"encoding/json"
	"strings"

	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

// Internal datastax DNS addresses
// to use for distros (like Alpine)
// that do not query DNS servers in order.
var DatastaxDns = []string{"10.100.6.66", "10.100.6.67"}

type DockerCmd struct {
	Args      []string
	ConfigDir string
	Input     string
}

func (cmd DockerCmd) ToCliArgs() []string {
	var args []string
	if cmd.ConfigDir != "" {
		args = append(args, "--config", cmd.ConfigDir)
	}
	args = append(args, cmd.Args...)
	return args
}

func (cmd DockerCmd) WithCfg(cfgDir string) DockerCmd {
	cmd.ConfigDir = cfgDir
	return cmd
}

func (cmd DockerCmd) WithInput(in string) DockerCmd {
	cmd.Input = in
	return cmd
}

func FromArgs(args []string) DockerCmd {
	return DockerCmd{Args: args}
}

func FromArgsInput(args []string, in string) DockerCmd {
	return DockerCmd{Args: args, Input: in}
}

func exec(cmd DockerCmd,
	withInput func(string, string, ...string) error,
	withoutInput func(string, ...string) error) error {

	var err error
	args := cmd.ToCliArgs()
	if cmd.Input != "" {
		err = withInput("docker", cmd.Input, args...)

	} else {
		err = withoutInput("docker", args...)
	}
	return err
}

func output(cmd DockerCmd,
	withInput func(string, string, ...string) (string, error),
	withoutInput func(string, ...string) (string, error)) (string, error) {

	var err error
	var out string
	args := cmd.ToCliArgs()
	if cmd.Input != "" {
		out, err = withInput("docker", cmd.Input, args...)

	} else {
		out, err = withoutInput("docker", args...)
	}
	return out, err
}

func (cmd DockerCmd) Exec() error {
	return exec(cmd, shutil.RunWithInput, shutil.Run)
}

func (cmd DockerCmd) ExecV() error {
	return exec(cmd, shutil.RunVWithInput, shutil.RunV)
}

func (cmd DockerCmd) ExecVPanic() {
	err := exec(cmd, shutil.RunVWithInput, shutil.RunV)
	mageutil.PanicOnError(err)
}

func (cmd DockerCmd) Output() (string, error) {
	return output(cmd, shutil.OutputWithInput, shutil.Output)
}

func (cmd DockerCmd) OutputPanic() string {
	out, err := output(cmd, shutil.OutputWithInput, shutil.Output)
	mageutil.PanicOnError(err)
	return out
}

func Exec(containerId string, env []string, privileged bool, user string, workDir string, exec []string) DockerCmd {
	args := []string{"exec"}

	for _, x := range env {
		args = append(args, "--env")
		args = append(args, x)
	}
	if privileged {

		args = append(args, "--privileged")
	}
	if user != "" {
		args = append(args, "--user", user)
	}
	if workDir != "" {
		args = append(args, "--workdir", workDir)
	}
	args = append(args, containerId)
	args = append(args, exec...)
	return FromArgs(args)
}

func Run(imageName string, volumes []string, dnsAddrs []string, env []string, runArgs []string, execArgs []string) DockerCmd {
	args := []string{"run", "--rm"}

	for _, x := range env {
		args = append(args, "--env")
		args = append(args, x)
	}
	for _, x := range dnsAddrs {
		args = append(args, "--dns")
		args = append(args, x)
	}
	for _, x := range volumes {
		args = append(args, "-v")
		args = append(args, x)
	}
	args = append(args, runArgs...)
	args = append(args, imageName)
	args = append(args, execArgs...)
	return FromArgs(args)
}

func Build(contextDir string, targetStage string, dockerFilePath string, namesAndTags []string, buildArgs []string) DockerCmd {
	args := []string{"build", contextDir}

	if targetStage != "" {
		args = append(args, "--target")
		args = append(args, targetStage)
	}
	if dockerFilePath != "" {
		args = append(args, "-f")
		args = append(args, dockerFilePath)
	}
	for _, x := range namesAndTags {
		args = append(args, "-t")
		args = append(args, x)
	}
	for _, x := range buildArgs {
		args = append(args, "--build-arg")
		args = append(args, x)
	}
	return FromArgs(args)
}

func Login(configDir string, user string, pw string, repo string) DockerCmd {
	args := []string{
		"login", "-u", user,
		"--password-stdin",
		repo,
	}
	return FromArgsInput(args, pw)
}

func Push(tag string) DockerCmd {
	args := []string{"push", tag}
	return FromArgs(args)
}

func Tag(src string, target string) DockerCmd {
	args := []string{"tag", src, target}
	return FromArgs(args)
}

type Container struct {
	Id           string `json:"ID"`
	Image        string `json:"Image"`
	Command      string `json:"Command"`
	CreatedAt    string `json:"CreatedAt"`
	RunningFor   string `json:"RunningFor"`
	Status       string `json:"Status"`
	Size         string `json:"Size"`
	Names        string `json:"Names"`
	Labels       string `json:"Labels"`
	Mounts       string `json:"Mounts"`
	Networks     string `json:"Networks"`
	Ports        string `json:"Ports"`
	LocalVolumes string `json:"LocalVolumes"`
}

func LsContainers(format string) DockerCmd {
	args := []string{
		"container", "ls",
		"--format", format,
		"--no-trunc",
	}
	return FromArgs(args)
}

func GetAllContainersPanic() []Container {
	var containers []Container
	format := "\"{{json .}}\""
	rawJson := LsContainers(format).OutputPanic()
	for _, j := range strings.Split(rawJson, "\n") {
		var c Container
		//trim extra quotes around the json
		js := strings.Trim(j, "\"")
		err := json.Unmarshal([]byte(strings.TrimSpace(js)), &c)
		mageutil.PanicOnError(err)
		containers = append(containers, c)
	}
	return containers
}
