FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:08d4c70f1baf6537a0728656cf7942d561e74a35916509d205b6519861266352
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
