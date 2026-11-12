FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:baafce2d34783a28b4a472779f304e97e00f9d35c99553bc53b068b14344d9a9
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
