package fallout

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	dockerutil "github.com/riptano/dse-operator/mage/docker"
	gitutil "github.com/riptano/dse-operator/mage/git"
	shutil "github.com/riptano/dse-operator/mage/sh"
	mageutil "github.com/riptano/dse-operator/mage/util"
)

const (
	imageName        = "fallout_dse_operator"
	testMount        = "/tests"
	buildMount       = "/build"
	testDir          = "fallout/tests"
	buildDir         = "fallout/build"
	envOperatorImage = "M_OPERATOR_IMAGE"
	envDseImage      = "M_DSE_IMAGE"
	envTestFile      = "M_FTEST"
	envTestRuns      = "M_RUNS"
	envGithubToken   = "GITHUB_TOKEN"
	envFalloutToken  = "FALLOUT_OAUTH_TOKEN"
	queuedRunsFile   = "queuedruns.txt"
)

type testRun struct {
	id     string
	name   string
	status string
	params []string
}

func writeBuildFile(fileName string, contents string) {
	outputPath := filepath.Join(buildDir, fileName)
	err := ioutil.WriteFile(outputPath, []byte(contents), 0666)
	if err != nil {
		fmt.Printf("Failed to write file at %s\n", outputPath)
		panic(err)
	}
	//For some reason, this step is necessary to actually
	//get the expected permissions
	os.Chmod(outputPath, 0666)
}

func (test *testRun) serialize() string {
	return fmt.Sprintf("%s:%s", test.name, test.id)
}

func serializeTestRuns(tests []testRun) string {
	var result string
	for _, test := range tests {
		s := test.serialize()
		if result == "" {
			result = fmt.Sprintf("%s", s)
		} else {
			result = fmt.Sprintf("%s,%s", result, s)
		}
	}
	return result
}

func stripExtension(fileName string) string {
	if !strings.HasSuffix(fileName, ".yaml") {
		msg := fmt.Errorf("%s invalid test file name. File must end in .yaml\n", fileName)
		panic(msg)
	}
	return fileName[:strings.IndexByte(fileName, '.')]
}

func monitorTestRunChannel(c chan testRun, count int, callBack func(testRun)) []testRun {
	var tests []testRun
	for i := 0; i < count; i++ {
		t := <-c
		callBack(t)
		tests = append(tests, t)
	}
	return tests
}

func fallout(args []string) string {
	mageutil.EnsureDir(buildDir)
	cwd, err := os.Getwd()
	mageutil.PanicOnError(err)

	localTestDir := fmt.Sprintf("%s/%s", cwd, testDir)
	localBuildDir := fmt.Sprintf("%s/%s", cwd, buildDir)

	volumes := []string{
		fmt.Sprintf("%s:%s", localTestDir, testMount),
		fmt.Sprintf("%s:%s", localBuildDir, buildMount),
	}

	fallout_token := mageutil.RequireEnv(envFalloutToken)
	env := []string{fmt.Sprintf("FALLOUT_OAUTH_TOKEN=%s", fallout_token)}
	execArgs := append([]string{"fallout"}, args...)
	dockerCmd := dockerutil.Run(imageName, volumes, dockerutil.DatastaxDns, env, []string{}, execArgs)
	return dockerCmd.OutputPanic()
}

// Only discover tests directly sitting in the tests dir
func discoverTests() []string {
	var tests []string

	results, err := ioutil.ReadDir(testDir)
	mageutil.PanicOnError(err)
	for _, info := range results {
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".yaml") {
			tests = append(tests, info.Name())
		}
	}

	return tests
}

func getLinkToTestRun(test testRun) string {
	// The (seemingly) only way to find out what user
	// is associate with a testrun is to use the cli to
	// query info on the test itself, which will spit out
	// the test yaml + an extra set of comment headers that
	// list the user who created it.
	// The user is required to build out a URL to the test-run.
	args := []string{"test-info", test.name}
	out := fallout(args)
	prefix := "### Created by "
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			user := strings.TrimPrefix(line, prefix)
			return fmt.Sprintf("https://fallout.sjc.dsinternal.org/tests/ui/%s/%s/%s/artifacts", user, test.name, test.id)
		}
	}
	return ""
}

//---------- Queueing tests
func queueTest(c chan testRun, fileName string) {
	hash := gitutil.GetLongHash("")
	image := os.Getenv(envOperatorImage)
	if len(image) == 0 {
		image = fmt.Sprintf("datastax-docker.jfrog.io/dse-operator/operator:%s", hash)
	}

	dseImage := os.Getenv(envDseImage)

	testName := stripExtension(fileName)
	params := []string{
		fmt.Sprintf("operator_image=%s", image),
		fmt.Sprintf("yaml_src_branch=%s", hash),
	}

	if len(dseImage) > 0 {
		params = append(params, fmt.Sprintf("dse_image=%s", dseImage))
	}

	execArgs := []string{"create-testrun", testName}
	execArgs = append(execArgs, params...)
	out := fallout(execArgs)
	data := strings.Fields(out)
	c <- testRun{name: data[0], id: data[1], params: params}
}

func queueMany(c chan testRun, testFiles []string) {
	for _, testFile := range testFiles {
		go queueTest(c, testFile)
	}
}

func queue(c chan testRun, testFiles []string) []testRun {
	var queuedTests []testRun
	fmt.Printf("- Queuing %d test(s)\n", len(testFiles))
	queueMany(c, testFiles)
	count := len(testFiles)
	counter := 0
	queuedTests = monitorTestRunChannel(c, count, func(t testRun) {
		//test name and id
		fmt.Printf("%d. %s (%s)\n", counter+1, t.name, t.id)
		//params used (if applicable)
		if len(t.params) > 0 {
			fmt.Println("- Params:")
			for _, p := range t.params {
				fmt.Printf("\t%s\n", p)
			}
		}
		//link to view the testrun on the fallout site
		link := getLinkToTestRun(t)
		if link != "" {
			fmt.Printf("- Testrun Url:\n\t%s\n", link)
		}
		counter++
	})
	return queuedTests
}

//---------- Waiting for tests
func waitForTestToFinish(c chan testRun, test testRun) {
	execArgs := []string{
		"testrun-info",
		"--wait",
		"--testrun=" + test.id,
		test.name}
	out := fallout(execArgs)
	data := strings.Fields(out)
	c <- testRun{name: data[0], id: data[1], status: data[2]}
}

func parseTestRuns(str string) []testRun {
	rawRuns := strings.Split(str, ",")
	var runs []testRun
	for _, run := range rawRuns {
		raw := strings.Split(run, ":")
		r := testRun{name: raw[0], id: raw[1]}
		runs = append(runs, r)
	}
	return runs
}

func wait(c chan testRun, runs []testRun) {
	fmt.Printf("- Waiting on %d test(s)\n", len(runs))
	for _, run := range runs {
		go waitForTestToFinish(c, run)
	}
	runCounter := 0
	failCounter := 0
	monitorTestRunChannel(c, len(runs), func(t testRun) {
		fmt.Printf("%d. %s: %s (%s)\n", runCounter+1, t.status, t.name, t.id)
		runCounter++
		if t.status != "PASSED" {
			failCounter++
		}
	})

	if failCounter != 0 {
		fmt.Printf("- %d test(s) were unsuccessful.\n", failCounter)
		os.Exit(1)
	}
}

//---------- Loading tests
func loadTest(fileName string) {
	execArgs := []string{
		"create-test",
		fmt.Sprintf("%s/%s", testMount, fileName),
	}
	fallout(execArgs)
}

func loadTests(files []string) {
	fmt.Printf("- Loading %d test(s)\n", len(files))
	for i, testName := range files {
		fmt.Printf("%d. %s\n", (i + 1), testName)
		loadTest(testName)
	}
}

//---------- Aborting tests
func abortTest(test testRun) {
	execArgs := []string{
		"abort-testrun",
		"--testrun=" + test.id,
		test.name}
	fmt.Printf("Aborting: %s (%s)\n", test.name, test.id)
	fallout(execArgs)
}

//---------- Artifacts

func downloadArtifactForRun(run testRun) {
	fmt.Printf("- Retrieving artifacts for %s (%s)\n", run.name, run.id)
	execArgs := []string{
		"testrun-artifact",
		"--testrun=" + run.id,
		run.name,
	}
	fallout(execArgs)

	pattern := filepath.Join(buildDir, "*", run.name, run.id)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println("Could not retrieve artifacts from build run.")
		panic(err)
	}

	if len(matches) == 1 {
		fmt.Printf("Artifacts downloaded at: %s\n", matches[0])
	}
}

func downloadArtifacts(runs []testRun) {
	for _, run := range runs {
		downloadArtifactForRun(run)
	}
}

//---------- Targets

// Download testrun artifacts
func Artifacts() {
	rawRuns := mageutil.RequireEnv(envTestRuns)
	runs := parseTestRuns(rawRuns)
	downloadArtifacts(runs)
}

// Uploads fallout tests to server;
// Specify a test file name in M_FTEST
// or leave empty to create & update all tests
func Load() {
	testFile := os.Getenv(envTestFile)
	var files []string
	if testFile == "" {
		files = discoverTests()
	} else {
		files = []string{testFile}
	}
	loadTests(files)
}

// Await fallout test results;
// Specify a serialized testrun string in M_RUNS
// or leave empty to wait for all tests
func Wait() {
	rawRuns := mageutil.RequireEnv(envTestRuns)
	runs := parseTestRuns(rawRuns)
	c := make(chan testRun)
	wait(c, runs)
}

// Enqueue fallout tests;
// Specify a test file name in M_FTEST
// or leave empty to run all tests
func Queue() {
	mg.Deps(Build)
	mg.Deps(Load)
	testFile := os.Getenv(envTestFile)
	c := make(chan testRun)
	var queuedTests []testRun
	var files []string
	if testFile == "" {
		files = discoverTests()
	} else {
		files = []string{testFile}
	}
	queuedTests = queue(c, files)
	s := serializeTestRuns(queuedTests) + "\n"
	fmt.Printf("Serialized queue string: %s", s)
	writeBuildFile(queuedRunsFile, s)
}

// Builds the docker image containing fallout-cli
func Build() {
	fmt.Println("- Building image:", imageName)
	github_token := mageutil.RequireEnv(envGithubToken)
	dockerArgs := []string{
		"build", "./fallout", "-t", imageName, "--build-arg", "GITHUB_TOKEN=" + github_token,
	}
	shutil.RunVPanic("docker", dockerArgs...)
	fmt.Println("Success")
}

// Cancel fallout tests;
// Specify a serialized testrun string in M_RUNS
func Abort() {
	mg.Deps(Build)
	envRuns := mageutil.RequireEnv(envTestRuns)
	testRuns := parseTestRuns(envRuns)
	for _, run := range testRuns {
		abortTest(run)
	}
}

// Run fallout tests;
// Specify a test file name in M_FTEST
// or leave empty to run all tests
func Test() {
	mg.Deps(Build)
	mg.Deps(Load)

	testFile := os.Getenv(envTestFile)
	c := make(chan testRun)
	var queuedTests []testRun
	var files []string
	if testFile == "" {
		files = discoverTests()
	} else {
		files = []string{testFile}
	}
	queuedTests = queue(c, files)
	wait(c, queuedTests)
}

// Remove the build directory used by fallout targets
func Clean() {
	os.RemoveAll(buildDir)
}
