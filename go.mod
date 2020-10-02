module github.com/crossplane/provider-rook

go 1.13

require (
	github.com/crossplane/crossplane v0.13.0-rc.0.20200923162121-6e6cee50d87f
	github.com/crossplane/crossplane-runtime v0.9.1-0.20201001234308-3f1afd106a8c
	github.com/crossplane/crossplane-tools v0.0.0-20201001224552-fb258cc0eb30
	github.com/google/go-cmp v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/rook/rook v1.1.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.2.4
)
