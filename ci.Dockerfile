FROM registry.access.redhat.com/ubi9/ubi-minimal
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
