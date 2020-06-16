# How to make a release

* Make a `release` branch.
* Update `buildsettings.yaml` in the root of the repo with `prerelease: rc1`
* Update `image: "datastax/cass-operator:1.2.3"` in the Helm `values.yaml`
* Make sure `hack/release/build-kind-control-planes.sh` has run and updated the yaml bundles
* Run all int tests
* Update `CHANGELOG.md`
* Update `README.md` links/references to the right version in URLs like `https://.../datastax/cass-operator/v1.2.3/...`
* Make a _temporary_ branch with the same thing you want to tag the release with, like `v4.5.6`. This enables all of the github URLs to work.
* Push the operator container up to dockerhub.
* Run through the three line example at the top of the readme, with only copy paste. Check that it works.
