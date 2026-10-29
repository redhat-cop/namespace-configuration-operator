FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:bba2b4bbf0ed15cfe7dd72a1e17ae7d2015a141f306b304e1a2785c41400e39c
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
