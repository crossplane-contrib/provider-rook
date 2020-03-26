module github.com/crossplane/provider-rook

go 1.13

require (
	github.com/crossplane/crossplane v0.9.0
	github.com/crossplane/crossplane-runtime v0.6.0
	github.com/crossplane/crossplane-tools v0.0.0-20200303232609-b3831cbb446d
	github.com/google/go-cmp v0.3.1
	github.com/pkg/errors v0.8.1
	github.com/rook/rook v1.1.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v0.17.3
	sigs.k8s.io/controller-runtime v0.4.0
	sigs.k8s.io/controller-tools v0.2.4
)
