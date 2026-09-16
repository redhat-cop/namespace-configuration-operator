FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:7b8e25a1b56ca4d00219198f3b5b51a3e1693a5c4f5369c5e190d7d6cb3f980e
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
