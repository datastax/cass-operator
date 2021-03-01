set VERSIONS 1.0.0 1.1.0 1.2.0 1.3.0 1.4.0 1.4.1 1.5.0 1.5.1
for VERSION in $VERSIONS
  echo "Version: $VERSION"
  echo "docker build -t bradfordcp/cass-operator-bundle:$VERSION -f bundle-$VERSION.Dockerfile ."
  docker build -t harbor.sjc.dsinternal.org/cass-operator/cass-operator-bundle:$VERSION -f bundle-$VERSION.Dockerfile .
  
  echo "docker push harbor.sjc.dsinternal.org/cass-operator/cass-operator-bundle:$VERSION"
  docker push harbor.sjc.dsinternal.org/cass-operator/cass-operator-bundle:$VERSION
end


set BUNDLELIST ""

for VERSION in $VERSIONS
  set BUNDLELIST $BUNDLELIST,harbor.sjc.dsinternal.org/cass-operator/cass-operator-bundle:$VERSION
end
# Remove ',' from start of bundlelist
set BUNDLELIST (string sub -s 2 $BUNDLELIST)

echo "opm index add --bundles $BUNDLELIST --tag harbor.sjc.dsinternal.org/cass-operator/cass-operator-index:latest -u docker"
opm index add --bundles $BUNDLELIST --tag harbor.sjc.dsinternal.org/cass-operator/cass-operator-index:latest -u docker
docker push harbor.sjc.dsinternal.org/cass-operator/cass-operator-index:latest
