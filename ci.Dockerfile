FROM registry.access.redhat.com/ubi9/ubi-minimal:9.6@sha256:34880b64c07f28f64d95737f82f891516de9a3b43583f39970f7bf8e4cfa48b7
WORKDIR /
COPY bin/manager .
USER 65532:65532

# Same production log defaults as the Dockerfile; without them main.go falls back to zap's
# development config (console encoder at debug). The shared workflow builds THIS file.
ENV ZAP_LOG_LEVEL=info
ENV ZAP_DEVEL=false

ENTRYPOINT ["/manager"]
