module github.com/crossplane/provider-rook

go 1.13

replace github.com/crossplane/crossplane => github.com/muvaf/crossplane v0.2.1-0.20200901093743-b633092bf61f

require (
	github.com/crossplane/crossplane v0.13.0-rc.0.20200901002346-79763d19e144
	github.com/crossplane/crossplane-runtime v0.9.1-0.20200831142237-1576699ee9ac
	github.com/crossplane/crossplane-tools v0.0.0-20200827141855-f51a6598f2bc
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
