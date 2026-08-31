FROM registry.access.redhat.com/ubi9/ubi-minimal@sha256:7fbeae18dc9476399f565e68255f602a3374ea8614ba3d14843565131a13ff93
WORKDIR /
COPY bin/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
