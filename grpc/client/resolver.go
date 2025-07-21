package main

import (
	"google.golang.org/grpc/resolver"
	"log"
)

const (
	myScheme      = "17x"
	myServiceName = "resolver.17x.lixueduan.com"
	backendAddr   = "localhost:8080"
)

type exampleResolverBuilder struct{}

func (*exampleResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &exampleResolver{
		target: target,
		cc:     cc,
		addrsStore: map[string][]string{
			myServiceName: {backendAddr},
		},
	}
	r.ResolveNow(resolver.ResolveNowOptions{})
	return r, nil
}
func (*exampleResolverBuilder) Scheme() string { return myScheme }

// exampleResolver is a
// Resolver(https://godoc.org/google.golang.org/grpc/resolver#Resolver).
type exampleResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

func (r *exampleResolver) ResolveNow(o resolver.ResolveNowOptions) {
	// 直接从map中取出对于的addrList
	log.Printf("Resolving target: %s", r.target.Endpoint())
	addrStrs := r.addrsStore[r.target.Endpoint()]
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[i] = resolver.Address{Addr: s}
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (*exampleResolver) Close() {}

func init() {
	// Register the example ResolverBuilder. This is usually done in a package's
	// init() function.
	resolver.Register(&exampleResolverBuilder{})
}
