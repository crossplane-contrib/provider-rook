module github.com/crossplane/provider-rook

go 1.13

require (
	github.com/crossplane/crossplane v0.11.0-rc.0.20200518221518-c795a3103d7f
	github.com/crossplane/crossplane-runtime v0.8.1-0.20200512204508-290de7349949
	github.com/crossplane/crossplane-tools v0.0.0-20200412230150-efd0edd4565b
	github.com/google/go-cmp v0.4.0
	github.com/pkg/errors v0.8.1
	github.com/rook/rook v1.1.2
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.2
	sigs.k8s.io/controller-runtime v0.6.0
	sigs.k8s.io/controller-tools v0.2.4
)
