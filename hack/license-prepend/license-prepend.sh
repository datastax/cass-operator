#!/bin/bash

set -euf -o pipefail
set -x

filelist=$(mktemp)

git ls-files | fgrep '.go' > "$filelist"

for i in $(cat "$filelist")
do
  if ! grep -q 'Please see the included license file for details' "$i"
  then
    cat hack/license-prepend/license-header.txt "$i" > "$i.new" && mv "$i.new" "$i"
  fi
done
