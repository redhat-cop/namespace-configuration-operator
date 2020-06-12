module github.com/redhat-cop/namespace-configuration-operator

require (
	github.com/openshift/api v0.0.0-20200205133042-34f0ec8dab87
	github.com/operator-framework/operator-sdk v0.18.1
	github.com/pkg/errors v0.9.1
	github.com/redhat-cop/operator-utils v0.2.5
	github.com/scylladb/go-set v1.0.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0

)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)

go 1.13
