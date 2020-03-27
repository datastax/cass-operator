#!/bin/bash
all_ids=(`docker container ls | egrep kindest | cut -d ' ' -f 1`)
for id in ${all_ids[*]}
do
   echo "Deleting old operator Docker image from Docker container: $id"
   docker exec $id crictl rmi docker.io/datastax/cass-operator:latest
done
echo "Loading new operator Docker image into KIND cluster"
kind load docker-image datastax/cass-operator:latest
echo "Done."
