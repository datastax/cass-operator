// Copyright DataStax, Inc.
// Please see the included license file for details.

package jenkins

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	cfgutil "github.com/datastax/cass-operator/mage/config"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
	"github.com/magefile/mage/mg"
	"gopkg.in/yaml.v2"
)

const (
	jenkinsUrl            = "https://jenkins-container-tooling.build.dsinternal.org/"
	jenkinsSshAddr        = "jenkins-container-tooling-ssh.build.dsinternal.org"
	jenkinsApiToken       = "MJ_TOKEN"
	jenkinsUserName       = "MJ_USER"
	jenkinsSshUser        = "MJ_SSHUSER"
	endpointPluginInstall = "pluginManager/installNecessaryPlugins"
	endpointJcasc         = "configuration-as-code/apply"
	endpointSafeRestart   = "safeRestart"
	// file paths are relative to repository root, where the magefile exists
	// that imports this library
	masterConfigYaml       = "./jenkins/master/masterConfig.yaml"
	jobsDir                = "./jenkins/master/jobs"
	viewsDir               = "./jenkins/master/views"
	buildDir               = "./jenkins/master/build"
	localEtcDefaultJenkins = "./jenkins/master/etc-default-jenkins.conf"
	etcDefaultJenkins      = "/etc/default/jenkins"
)

type plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type availableUpdates struct {
	Updates []plugin `json:"updates"`
}

func getFromJenkins(endpoint string) *http.Response {
	ep := fmt.Sprintf("%s/%s", jenkinsUrl, endpoint)
	user := mageutil.RequireEnv(jenkinsUserName)
	token := mageutil.RequireEnv(jenkinsApiToken)

	client := &http.Client{}
	req, _ := http.NewRequest("GET", ep, nil)
	req.SetBasicAuth(user, token)
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	return res
}

func validateAndGetBody(res *http.Response) []byte {
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusOK {
		msg := fmt.Errorf("Error: Status code %d:\n\t%s\n", res.StatusCode, body)
		panic(msg)
	}
	return body
}

func postToJenkins(endpoint string, body io.Reader) *http.Response {
	ep := fmt.Sprintf("%s/%s", jenkinsUrl, endpoint)
	user := mageutil.RequireEnv(jenkinsUserName)
	token := mageutil.RequireEnv(jenkinsApiToken)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", ep, body)
	req.SetBasicAuth(user, token)
	res, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	return res
}

func getPluginsWithUpdates() availableUpdates {
	endpoint := "updateCenter/coreSource/api/json?tree=updates[name,version]"
	res := getFromJenkins(endpoint)
	body := validateAndGetBody(res)
	var avail availableUpdates
	err := json.Unmarshal(body, &avail)
	if err != nil {
		panic(err)
	}
	return avail
}

func getPluginsFromConfig() []plugin {
	settings := cfgutil.ReadBuildSettings()
	var plugins []plugin
	for _, p := range settings.Jenkins.Master.Plugins {
		plugins = append(plugins, plugin{Name: p, Version: "latest"})
	}
	return plugins
}

func installPlugin(p plugin) {
	fmt.Printf("Installing plugin: %s, version: %s\n", p.Name, p.Version)
	body := fmt.Sprintf("<install plugin=\"%s@%s\" />", p.Name, p.Version)
	res := postToJenkins(endpointPluginInstall, bytes.NewBufferString(body))
	//ensure we have an OK status
	validateAndGetBody(res)
}

func getJenkinsSshUser() string {
	sshUser := os.Getenv(jenkinsSshUser)
	if sshUser == "" {
		fmt.Printf("%s not set, defaulting to ubuntu as ssh user\n", jenkinsSshUser)
		sshUser = "ubuntu"
	}
	return sshUser
}

func execJenkinsSsh(user string, command string) {
	userhost := fmt.Sprintf("%s@%s", user, jenkinsSshAddr)
	fmt.Printf("- Remotely Executing: %s\n", command)
	shutil.RunVPanic("ssh", userhost, command)
}

// The Jcasc job yaml structure needs to represent:
// jobs:
//   - script: |
//       ...groovy job config
type jobsYaml map[string][]map[string]string

func formatJobForYaml(job string) jobsYaml {
	script := make(map[string]string)
	script["script"] = job
	scripts := []map[string]string{script}
	jobs := make(jobsYaml)
	jobs["jobs"] = scripts
	return jobs
}

func listGroovyFiles(directory string) []string {
	contents, err := ioutil.ReadDir(directory)
	if err != nil {
		panic(err)
	}

	var groovyFiles []string
	for _, info := range contents {
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".groovy") {
			groovyFiles = append(groovyFiles, info.Name())
		}
	}
	return groovyFiles
}

func discoverJobs() []string {
	return listGroovyFiles(jobsDir)
}

func discoverViews() []string {
	return listGroovyFiles(viewsDir)
}

func postJobsToJenkins(job jobsYaml) {
	yml, err := yaml.Marshal(job)
	if err != nil {
		panic(err)
	}

	res := postToJenkins(endpointJcasc, bytes.NewBuffer(yml))
	body := string(validateAndGetBody(res))
	if body != "" {
		fmt.Println(body)
	}
}

//--------- Targets

// Update the Jenkins Config as Code config on the Jenkins Master.
//
// We use Jenkins Config as Code to manage the configuration of the Jenkins
// master from source control. This target updates the configuration of the
// Jenkins master based on the contents of masterConfig.yaml.
//
// Note that not all config is managed this way. In particular, secrets are
// deliberately omitted from masterConfig.yaml so they don't get checked into
// source code. It's assumed that the necessary secrets have been manually
// added to the master prior to running this target. Job definitions are also
// not currently present in the JCasC yaml, but may be in the future.
//
// When configuring a new plugin or feature, it's not necessary to know the
// necessary yaml structure. It is possible to edit the config via the Jenkins
// UI, and then examine the exported JCasC at:
// https://jenkins-container-tooling.build.dsinternal.org/configuration-as-code/
// It's common to extract config snippets this way to check into source control.
//
// This target uses the Jenkins API and MJ_USER and MJ_TOKEN must be set.
func UpdateJcasc() {
	fmt.Printf("- Updating master config from file: %s\n", masterConfigYaml)
	file, err := os.Open(masterConfigYaml)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	res := postToJenkins(endpointJcasc, file)
	body := string(validateAndGetBody(res))
	if body != "" {
		fmt.Println(body)
	}
}

// Deploy config files to the Jenkins master over scp.
//
// Some parameters can only be specified via configuration files on the Jenkins
// master, not via APIs. This target deploys those files to the Jenkins master
// after backing them up locally.
func UpdateConfigFiles() {
	mageutil.EnsureDir(buildDir)

	// Backup existing config
	sshUser := getJenkinsSshUser()
	remoteFile := fmt.Sprintf("%s@%s:%s",
		sshUser,
		jenkinsSshAddr,
		etcDefaultJenkins)
	backupFile := fmt.Sprintf("%s/etc-default-jenkins.%s",
		buildDir,
		time.Now().Format(time.RFC3339))
	fmt.Printf("- Backing up %s to %s\n", remoteFile, backupFile)
	shutil.RunVPanic("scp", remoteFile, backupFile)

	// scp to a tmpfile in $HOME, unless sshUser is root we don't have
	// permissions to scp directly to a system location
	localFile := localEtcDefaultJenkins
	tmpFile := fmt.Sprintf(".etc-default-jenkins.mage_tmp_%s",
		mageutil.RandomHex(16))
	remoteTmpFile := fmt.Sprintf("%s@%s:%s",
		sshUser,
		jenkinsSshAddr,
		tmpFile)
	fmt.Printf("- Copying %s to %s via SSH\n", localFile, remoteTmpFile)
	shutil.RunVPanic("scp", localFile, remoteTmpFile)

	// Escalate privileges to mv our tempfile to its final destination
	mvCommand := fmt.Sprintf("sudo mv %s %s", tmpFile, etcDefaultJenkins)
	execJenkinsSsh(sshUser, mvCommand)
}

// Update OS packages on the Jenkins master, including the Jenkins package.
//
// This SSH's into the Jenkins master to perform an apt upgrade. It assumes that
// you have passwordless SSH login to the Jenkins master configured, and that
// your remote account on the Jenkins master has passwordless sudo.
//
// By default, the "ubuntu" user is used to log in. This can be customized via
// the MJ_SSHUSER environment variable.
func UpdateSystem() {
	fmt.Println("- Updating master jenkins version to latest")
	sshUser := getJenkinsSshUser()

	update := "sudo apt update -q"
	execJenkinsSsh(sshUser, update)

	noPromptArgs := []string{
		"-y",
		"-qq",
		"-o",
		"Dpkg::Options::=--force-confdef",
		"-o",
		"Dpkg::Options::=--force-confold",
	}
	noPrompt := strings.Join(noPromptArgs, " ")
	upgrade := fmt.Sprintf("DEBIAN_FRONTEND=noninteractive sudo apt upgrade %s", noPrompt)
	execJenkinsSsh(sshUser, upgrade)
}

// Ensure required plugins are installed on the Jenkins master.
//
// We rely on a number of plugins to be present on the Jenkins master, and this
// target ensures they are present.
//
// To add/remove a plugin from the list of managed plugins, update the
// basePlugins list in mage/jenkins/lib.go. Note that not every plugin on the
// Jenkins server is managed in this list, many came pre-installed and aren't
// managed by this target. It's always reasonable to start managing a plugin
// that is on the Jenkins master via the target, even if it was installed
// some other way.
//
// This target uses the Jenkins API and MJ_USER and MJ_TOKEN must be set.
func InstallPlugins() {
	for _, p := range getPluginsFromConfig() {
		installPlugin(p)
	}
}

// Update all plugins on the Jenkins master.
//
// We rely on a number of plugins to be present on the Jenkins master, and this
// target ensures they are updated to the most recent release. This updates ALL
// plugins on the Jenkins master, not only those managed by the InstallPlugins
// target.
//
// This target uses the Jenkins API and MJ_USER and MJ_TOKEN must be set.
func UpdatePlugins() {
	fmt.Println("- Updating all plugins")
	avail := getPluginsWithUpdates()
	for _, p := range avail.Updates {
		installPlugin(p)
	}
}

// Perform all automated maintenance on the Jenkins Master.
//
// Installs plugins, Updates plugins, Updates the JCasC config, Updates job configs,
// and updates the system packages. Does not automatically restart the Jenkins Master
// as this is often unnecessary and can be disruptive.
func UpdateAll() {
	// Update the system first. The plugin stuff seems to be async. If you
	// watch /var/log/jenkins/jenkins.log on the master, you can see that
	// it's still doing plugin stuff well after the API call has returned.
	// Updating the jenkins package while that's happening seems
	// ill-advised.
	mg.Deps(UpdateConfigFiles)
	mg.Deps(UpdateSystem)
	mg.Deps(UpdateJcasc)
	// Jobs have to be added before they can be included in views, so we ensure
	// UpdateJobs happens before UpdateViews. The other way to do this is have
	// UpdateViews depend on UpdateJobs, but when working on adding new views, it's
	// nice to be able to update the views while leaving the jobs alone.
	mg.SerialDeps(UpdateJobs, UpdateViews)
	mg.Deps(InstallPlugins)
	mg.Deps(UpdatePlugins)
}

// Perform a 'safe restart' of the master jenkins server.
//
// It's sometimes necessary to restart the Jenkins master after performing
// maintenance, this target helps to do so. If this is necessary, the Jenkins
// UI will notify you.
//
// This target uses the Jenkins API and MJ_USER and MJ_TOKEN must be set.
func Restart() {
	postToJenkins(endpointSafeRestart, nil)
}

// Update job configurations on the Jenkins master.
//
// All .groovy files in the jenkins/master/jobs directory will be processed
// and posted to the master server through the JCasC endpoint.
//
// The Job-dsl plugin allows JCasC to accept job configurations. The groovy syntax
// used to define these configurations is documented at:
// https://jenkins-container-tooling.build.dsinternal.org/plugin/job-dsl/api-viewer/index.html
func UpdateJobs() {
	jobs := discoverJobs()
	fmt.Printf("- Loading %v job configurations\n", len(jobs))
	for _, job := range jobs {
		fmt.Printf("%s\n", job)
		path := fmt.Sprintf("%s/%s", jobsDir, job)
		job, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		formatted := formatJobForYaml(string(job))
		postJobsToJenkins(formatted)
	}
}

// Update job views on the Jenkins master.
func UpdateViews() {
	views := discoverViews()
	fmt.Printf("- Loading %v view configurations\n", len(views))
	for _, view := range views {
		fmt.Printf("%s\n", view)
		path := fmt.Sprintf("%s/%s", viewsDir, view)
		view, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		formatted := formatJobForYaml(string(view))
		postJobsToJenkins(formatted)
	}
}

// Remove the build directory used by jenkins targets
func Clean() {
	os.RemoveAll(buildDir)
}
