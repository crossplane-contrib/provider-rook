module github.com/crossplaneio/stack-rook

go 1.12

replace (
	github.com/crossplaneio/crossplane => github.com/negz/crossplane v0.1.1-0.20191021220231-e9e873bf2ab7a9248492fea4d1713b2a391832c4
	github.com/crossplaneio/crossplane-runtime => github.com/negz/crossplane-runtime v0.0.0-20191021215706-aafb5162c8cd
)

require (
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/crossplaneio/crossplane v0.0.0-00010101000000-000000000000
	github.com/crossplaneio/crossplane-runtime v0.0.0-20190915084059-26a458d08504
	github.com/google/go-cmp v0.3.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/onsi/ginkgo v1.10.1 // indirect
	github.com/onsi/gomega v1.7.0 // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/pkg/errors v0.8.1
	github.com/rook/rook v1.1.2
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.4 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v1.0.0 // indirect
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf // indirect
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4 // indirect
	sigs.k8s.io/controller-runtime v0.2.0
)
