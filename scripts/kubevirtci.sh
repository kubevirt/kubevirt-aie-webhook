#!/bin/bash

set -ex

export KUBEVIRT_MEMORY_SIZE="${KUBEVIRT_MEMORY_SIZE:-16G}"
export KUBEVIRT_REPO="${KUBEVIRT_REPO:-https://github.com/kubevirt/kubevirt.git}"
export KUBEVIRT_BRANCH="${KUBEVIRT_BRANCH:-main}"
export NAMESPACE="${NAMESPACE:-kubevirt}"

_base_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
_kubevirt_dir="${_base_dir}/_kubevirt"
_cluster_up_dir="${_kubevirt_dir}/kubevirtci/cluster-up"
_kubectl="${_cluster_up_dir}/kubectl.sh"
_cli="${_cluster_up_dir}/cli.sh"
_action=$1
shift

function determine_cri_bin() {
    if [ "${KUBEVIRT_CRI}" = "podman" ]; then
        echo podman
    elif [ "${KUBEVIRT_CRI}" = "docker" ]; then
        echo docker
    else
        if podman ps >/dev/null 2>&1; then
            echo podman
        elif docker ps >/dev/null 2>&1; then
            echo docker
        else
            echo ""
        fi
    fi
}

function kubevirtci::install() {
    if [[ ! -d "${_kubevirt_dir}" ]]; then
        git clone --depth 1 --branch "${KUBEVIRT_BRANCH}" "${KUBEVIRT_REPO}" "${_kubevirt_dir}"
    fi
}

function kubevirtci::up() {
    make cluster-up -C "${_kubevirt_dir}"
    make cluster-sync -C "${_kubevirt_dir}"

    echo "waiting for kubevirt to become ready, this can take a few minutes..."
    ${_kubectl} -n "${NAMESPACE}" wait kv kubevirt --for condition=Available --timeout=15m
}

function kubevirtci::down() {
    make cluster-down -C "${_kubevirt_dir}"
}

function kubevirtci::sync() {
    local cri
    cri=$(determine_cri_bin)
    if [[ -z "${cri}" ]]; then
        echo >&2 "no working container runtime found. Neither docker nor podman seems to work."
        exit 1
    fi

    local docker_tag="${DOCKER_TAG:-devel}"
    local docker_prefix="${DOCKER_PREFIX:-quay.io/kubevirt}"
    local image_name="${IMAGE_NAME:-kubevirt-aie-webhook}"
    local img="${docker_prefix}/${image_name}:${docker_tag}"

    echo "Building container image ${img}..."
    ${cri} build -t "${img}" "${_base_dir}"

    echo "Loading image into cluster..."
    local registry_port
    registry_port=$(${_cli} ports registry 2>/dev/null || true)
    if [[ -n "${registry_port}" ]]; then
        local registry="localhost:${registry_port}"
        local registry_img="${registry}/${image_name}:${docker_tag}"
        ${cri} tag "${img}" "${registry_img}"
        ${cri} push "${registry_img}" --tls-verify=false 2>/dev/null || \
            ${cri} push "${registry_img}"
        img="${registry_img}"
    fi

    echo "Deploying webhook to cluster..."
    local kubeconfig
    kubeconfig=$(kubevirtci::kubeconfig)

    KUBECONFIG="${kubeconfig}" helm upgrade --install kubevirt-aie-webhook \
        "${_base_dir}/deploy/helm/kubevirt-aie-webhook" \
        --namespace "${NAMESPACE}" \
        --create-namespace \
        --set image.repository="${img%:*}" \
        --set image.tag="${docker_tag}" \
        --set image.pullPolicy="${IMAGE_PULL_POLICY:-Always}" \
        --set namespace="${NAMESPACE}" \
        --wait

    echo "Webhook synced to cluster."
}

function kubevirtci::kubeconfig() {
    "${_cluster_up_dir}/kubeconfig.sh"
}

function kubevirtci::kubectl() {
    ${_kubectl} "$@"
}

kubevirtci::install

case ${_action} in
    "up")
        kubevirtci::up
        ;;
    "down")
        kubevirtci::down
        ;;
    "sync")
        kubevirtci::sync
        ;;
    "kubeconfig")
        kubevirtci::kubeconfig
        ;;
    "kubectl")
        kubevirtci::kubectl "$@"
        ;;
    *)
        echo "Unknown command '${_action}'. Known commands: up, down, sync, kubeconfig, kubectl"
        exit 1
        ;;
esac
