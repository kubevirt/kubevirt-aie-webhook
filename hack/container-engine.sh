#!/bin/bash
if [ -n "${KUBEVIRT_CRI}" ]; then
    echo "${KUBEVIRT_CRI}"
elif podman ps >/dev/null 2>&1; then
    echo podman
elif docker ps >/dev/null 2>&1; then
    echo docker
fi
