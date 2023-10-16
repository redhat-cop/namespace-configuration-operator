module github.com/redhat-cop/namespace-configuration-operator

go 1.16

require (
	github.com/go-logr/logr v1.2.4
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.27.10
	github.com/openshift/api v3.9.0+incompatible
	github.com/redhat-cop/operator-utils v1.2.2
	github.com/scylladb/go-set v1.0.2
	k8s.io/api v0.28.1
	k8s.io/apimachinery v0.28.1
	k8s.io/client-go v0.28.1
	sigs.k8s.io/controller-runtime v0.16.2
)
