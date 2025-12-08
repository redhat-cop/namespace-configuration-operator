apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: configure-operator-log-level
  annotations:
    policies.kyverno.io/title: Configure Namespace Configuration Operator Log Level
    policies.kyverno.io/category: Operator Configuration
    policies.kyverno.io/severity: low
    policies.kyverno.io/subject: Deployment
    pod-policies.kyverno.io/autogen-controllers: none
    policies.kyverno.io/description: >-
      Injects log level environment variables into the namespace-configuration-operator
      Deployment. This policy works with OLM-managed deployments and ensures log level
      configuration persists even when OLM updates the Deployment.
spec:
  background: false
  rules:
    - name: inject-log-level-env
      match:
        any:
          - resources:
              kinds:
                - Deployment
              names:
                - namespace-configuration-operator-controller-manager
              namespaces:
                - namespace-configuration-operator
              operations:
                - CREATE
                - UPDATE
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                containers:
                  - name: manager
                    env:
                      # Configure via environment variables:
                      # - ZAP_LOG_LEVEL: "error" | "info" | "debug" | "0-10"
                      #   - "error" = only errors
                      #   - "info" = info and above (recommended for production)
                      #   - "debug" = debug and above
                      #   - "2" = verbosity level 2 (shows template filtering logs)
                      # - ZAP_DEVEL: "true" | "false"
                      #   - "false" = JSON format (production)
                      #   - "true" = console format (development)
                      - name: ZAP_LOG_LEVEL
                        value: "${ZAP_LOG_LEVEL}"
                      - name: ZAP_DEVEL
                        value: "${ZAP_DEVEL}"

