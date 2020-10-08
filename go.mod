module github.com/crossplane/provider-rook

go 1.13

require (
	github.com/crossplane/crossplane v0.13.0-rc.0.20200923162121-6e6cee50d87f
	github.com/crossplane/crossplane-runtime v0.9.1-0.20201008065523-51c117eff562
	github.com/crossplane/crossplane-tools v0.0.0-20201007233256-88b291e145bb
	github.com/google/go-cmp v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/rook/rook v1.1.2
	golang.org/x/tools v0.0.0-20200410194907-79a7a3126eef // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.2.4
)
