#!/usr/bin/env bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

function log_err() {
    echo $@ 1>&2
}

function extract_snippet() {
    # Code snippets look like:
    #
    #    <!-- SNIPPET: some-snippet -->
    #    ```
    #    ... some code ...
    #    ```
    #
    # This function returns the '... some code ...' bit.

    local readme_file="$1"
    local snippet_name="$2"

    # I'm sure there's a mind-blowing sed oneliner that does what we want, but
    # I have no idea what it is.

    # Find line snippet starts on by looking for <!-- SNIPPET: <some_name> -->
    local line_start=$(\
        grep --extended-regexp \
            --line-number \
            "[<][!][-][-]\s*SNIPPET[:]\s*$snippet_name\s*[^>]*[>]" "$readme_file" \
            | cut -f1 -d:)

    if [[ "$(echo "$line_start" | wc -l)" -gt 1 ]]; then
        log_err "More than one SNIPPET with name '$snippet_name'. Found snippet on lines: $line_start"
        return 1
    fi

    if [[ -z "$line_start" ]]; then
        log_err "Did not find snippet named '$snippet_name'"
        return 1
    fi

    # Get the position of opening ``` which will be right after line_start
    local opening_fence_line=$(expr 1 + "$line_start")

    # Get the position where the code starts, this will be right after the
    # opening ```
    local code_start=$(expr 1 + "$opening_fence_line")

    # tail -n +$code_start "$readme_file"
    # Now we need to find where the code ends, that will be the first occurence
    # of ``` after the opening ```.
    local closing_fence_offset=$(\
        tail -n +${code_start} "$readme_file" \
        | grep --extended-regexp --line-number '^[`][`][`]' \
        | cut -f1 -d: \
        | head -n 1)

    local code_length=$(expr $closing_fence_offset - 1)

    # Extract the lines of code
    tail -n +$code_start "$readme_file" | head -n "$code_length"
}

function eval_snippet() {
    local readme_file="$1"
    local snippet_name="$2"
    local snippet
    if snippet=$(extract_snippet "$readme_file" "$snippet_name"); then
        echo "Running snippet: $snippet_name"
        if ! eval "$snippet"; then
            log_err "Error occurred while executing snippet '$snippet_name'"
            return 1
        fi
    else
        return 1
    fi
}

function cluster_ready() {
    local cluster_name="$1"
    local node_count="$2"
    local ready_count="$(KUBECONFIG=kubeconfig_kubespray.conf kubectl get pods | grep "$cluster_name" | grep Running | grep "1/1" | wc -l)"
    [[ "$node_count" -eq "$ready_count" ]]
}

function wait_ready() {
    local attempt_count=0
    while ! cluster_ready c1 1; do
        sleep 10
        attempt_count=$((attempt_count+1))
        if [[ $attempt_count -gt 120 ]]; then
            log_err "Timed out waiting for cluster"
            return 1
        fi
    done

    return 0
}

function cleanup() {
    ctool destroy k8s || true
    ctool destroy k8s_kubespray || true
}

function ctool_cluster_exists() {
    local name="$1"
    ctool list \
        | grep --only-matching --extended-regexp '^[^,]+' \
        | grep --extended-regexp "^${name}\$"
}

function assert_ctool_clusters_gone() {
    for cluster_name in $@; do
        if ctool_cluster_exists "$cluster_name"; then
            return 1
        fi
    done
}

function assert_kubeconfig_exists() {
    [[ -e "kubeconfig_kubespray.conf" ]]
}

cd "$SCRIPT_DIR"

cleanup

README_FILE="${SCRIPT_DIR}/../docs/developer/kctool.md"

eval_snippet "${README_FILE}" "create-cluster"
eval_snippet "${README_FILE}" "get-kubeconfig"
assert_kubeconfig_exists
eval_snippet "${README_FILE}" "deploy-operator"
sleep 10
eval_snippet "${README_FILE}" "create-dse-datacenter"
wait_ready
eval_snippet "${README_FILE}" "delete"
assert_ctool_clusters_gone "kubespray" "k8s"

cleanup
