#!/usr/bin/env bash

GIT_TAG=$(git describe --exact-match --tags HEAD)

if [ -n "$GIT_TAG" ]; then
    echo "$GIT_TAG"
else
    echo "latest"
fi
