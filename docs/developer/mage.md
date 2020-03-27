# Introduction

Mage is a build tool for golang projects.

# Installation

Ensure that `$GOPATH/bin` directory is in your `$PATH`:
``` bash
$ echo $GOPATH
# /home/myhome/go

$ echo $PATH
# /usr/sbin:/usr/bin:/sbin:/bin:/home/myhome/go/bin

```

Follow the instructions at: https://magefile.org/#installation

# Documentation

* [Documentation](https://magefile.org/)
* [Github](https://github.com/magefile/mage)
* [Demo Video](https://www.youtube.com/watch?v=Hoga60EF_1U)

# Mage Tips

## List Targets
If you don't know what targets are available, mage will tell you.
```bash
$ mage
# Targets:
#  jenkins:updateSystem      Update OS packages on the Jenkins master, including the Jenkins package.
#  ...snip output...
#  operator:buildGo          Builds operator go code.


$ mage -l
# Targets:
#  ...same output as above...
```

## Target Help
If you don't know how to use a target, you can get detailed help. This is
broken pending [mage-249](https://github.com/magefile/mage/issues/249).
``` bash
$ mage -h jenkins:updateSystem     # pre mage-249
# Unknown target: "jenkins:updateSystem"

$ mage -h jenkins:updateSystem     # post mage-249
# Update OS packages on the Jenkins master, including the Jenkins package.
# This SSH's into the Jenkins master to perform an apt upgrade. It assumes that
# you have passwordless SSH login to the Jenkins master configured, and that
# your remote account on the Jenkins master has passwordless sudo.
```

When writing mage targets, add help output to them by adding a docstring to
the function that defines the target.
``` go
// The first sentence in the comment will be the text shown with mage -l.
//
// The rest of the comment is long help text that will be shown with
// mage -h <target>
func MyTarget() {
    log.Printf("Hi!")
}
```

## Target Parameters
Supply an ad-hoc environmental parameter to mage. Mage targets don't accept
command-line flags, all parameterization is done through environment variables.
In general, you'll want to modify your `~/.bashrc` or similar file to export
the relevant environment variables, but occassionally an one-time parameter
must be supplied.
``` bash
$ MJ_SSHUSER=myname mage jenkins:updateSystem
# ... snip ...
```

## Magefile Directory
Run mage from a different directory. Mage automatically looks for `magefile.go`
in the current working directory (cwd), which works without further
configuration from the project root directory. If you wish to execute mage
targets when your cwd is something other than the project root, you must point
mage to our `magefile.go`.
``` bash
$ ls magefile.go                   # magefile is in project root
# magefile.go

$ mage -l                          # automatically finds a magefile in the cwd
# Targets:
#  operator:buildGo                Builds operator go code.
#  ...snip output...

$ cd operator

$ ls magefile.go
# ls: cannot access 'magefile.go': No such file or directory

$ mage -l
# Error determining list of magefiles: failed to list mage gofiles: exit status 1:

$ mage -d .. -l
# Targets:
#  operator:buildGo                Builds operator go code.
#  ...snip output...
```
