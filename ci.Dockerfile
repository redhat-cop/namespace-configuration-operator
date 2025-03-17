FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:bafd57451de2daa71ed301b277d49bd120b474ed438367f087eac0b885a668dc
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
