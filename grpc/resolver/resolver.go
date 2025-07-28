package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.etcd.io/etcd/client/v3"
	etcdresolver "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	ecpb "test/grpc/hello"
)

const (
	customScheme = "custom-etcd"
	serviceKey   = "hello-service"
	backendAddr  = "localhost:8080"
)

// 服务信息结构
type ServiceInfo struct {
	Addr     string            `json:"addr"`
	Weight   int               `json:"weight"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// 自定义 resolver 构建器
type customEtcdResolverBuilder struct {
	etcdClient *clientv3.Client
}

func newCustomEtcdResolverBuilder(etcdClient *clientv3.Client) *customEtcdResolverBuilder {
	return &customEtcdResolverBuilder{
		etcdClient: etcdClient,
	}
}

func (b *customEtcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// 创建底层的 etcd resolver
	etcdResolverBuilder, err := etcdresolver.NewBuilder(b.etcdClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd resolver builder: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建自定义 resolver
	r := &customEtcdResolver{
		etcdClient:          b.etcdClient,
		etcdResolverBuilder: etcdResolverBuilder,
		target:              target,
		cc:                  cc,
		ctx:                 ctx,
		cancel:              cancel,
		addressCache:        make(map[string]ServiceInfo),
	}

	// 启动监听
	go r.start()
	return r, nil
}

func (b *customEtcdResolverBuilder) Scheme() string {
	return customScheme
}

// 自定义 resolver
type customEtcdResolver struct {
	etcdClient          *clientv3.Client
	etcdResolverBuilder resolver.Builder
	target              resolver.Target
	cc                  resolver.ClientConn
	ctx                 context.Context
	cancel              context.CancelFunc

	mu           sync.RWMutex
	addressCache map[string]ServiceInfo
}

func (r *customEtcdResolver) start() {
	// 构建服务的完整 etcd key
	servicePrefix := fmt.Sprintf("/services/%s/", r.target.Endpoint())

	// 立即解析一次
	r.ResolveNow(resolver.ResolveNowOptions{})

	// 监听 etcd 变化
	watchChan := r.etcdClient.Watch(r.ctx, servicePrefix, clientv3.WithPrefix())

	for {
		select {
		case <-r.ctx.Done():
			return
		case watchResp := <-watchChan:
			if watchResp.Err() != nil {
				log.Printf("Watch error: %v", watchResp.Err())
				continue
			}
			log.Println("Etcd services changed, updating addresses...")
			r.updateCache()
			r.ResolveNow(resolver.ResolveNowOptions{})
		}
	}
}

func (r *customEtcdResolver) updateCache() {
	servicePrefix := fmt.Sprintf("/services/%s/", r.target.Endpoint())

	resp, err := r.etcdClient.Get(r.ctx, servicePrefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("Failed to get services from etcd: %v", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 清空缓存
	r.addressCache = make(map[string]ServiceInfo)

	// 重新填充缓存
	for _, kv := range resp.Kvs {
		var info ServiceInfo
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			// 如果不是 JSON 格式，创建默认信息
			info = ServiceInfo{
				Addr:   string(kv.Value),
				Weight: 1,
			}
		}
		r.addressCache[string(kv.Key)] = info
	}
}

func (r *customEtcdResolver) ResolveNow(resolver.ResolveNowOptions) {
	r.updateCache()

	// TODO: 在这里实现你的选择策略
	// 目前返回所有可用地址
	addrs := r.selectAll()

	if len(addrs) > 0 {
		log.Printf("Selected addresses: %v", r.formatAddresses(addrs))
		r.cc.UpdateState(resolver.State{Addresses: addrs})
	} else {
		log.Printf("No addresses available")
	}
}

// 默认选择所有地址（可以根据需要修改此方法）
func (r *customEtcdResolver) selectAll() []resolver.Address {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var addrs []resolver.Address
	for _, info := range r.addressCache {
		addrs = append(addrs, resolver.Address{Addr: info.Addr})
	}

	return addrs
}

// 获取缓存的服务信息（供选择策略使用）
func (r *customEtcdResolver) getServiceInfos() map[string]ServiceInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 返回副本以避免并发问题
	result := make(map[string]ServiceInfo)
	for k, v := range r.addressCache {
		result[k] = v
	}
	return result
}

func (r *customEtcdResolver) formatAddresses(addrs []resolver.Address) []string {
	var result []string
	for _, addr := range addrs {
		result = append(result, addr.Addr)
	}
	return result
}

func (r *customEtcdResolver) Close() {
	r.cancel()
}

// 辅助函数：注册服务到 etcd
func registerService(etcdClient *clientv3.Client, serviceName, instanceID, addr string, weight int, metadata map[string]string) error {
	key := fmt.Sprintf("/services/%s/%s", serviceName, instanceID)

	info := ServiceInfo{
		Addr:     addr,
		Weight:   weight,
		Metadata: metadata,
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	_, err = etcdClient.Put(context.Background(), key, string(data))
	if err != nil {
		return fmt.Errorf("failed to register service: %v", err)
	}

	log.Printf("Registered service: %s -> %s", key, addr)
	return nil
}

func callUnaryEcho(c ecpb.HelloServiceClient, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &ecpb.HelloRequest{Name: message})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	fmt.Println(r.Message)
}

func makeRPCs(cc *grpc.ClientConn, n int) {
	hwc := ecpb.NewHelloServiceClient(cc)
	for i := 0; i < n; i++ {
		callUnaryEcho(hwc, fmt.Sprintf("request #%d", i+1))
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	// 创建 etcd 客户端
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	defer etcdClient.Close()

	// 注册服务实例到 etcd
	services := []struct {
		id       string
		addr     string
		weight   int
		metadata map[string]string
	}{
		{"instance1", "localhost:8080", 3, map[string]string{"region": "us-west", "zone": "a"}},
		{"instance2", "localhost:8081", 2, map[string]string{"region": "us-west", "zone": "b"}},
		{"instance3", "localhost:8082", 1, map[string]string{"region": "us-east", "zone": "a"}},
	}

	for _, svc := range services {
		if err := registerService(etcdClient, serviceKey, svc.id, svc.addr, svc.weight, svc.metadata); err != nil {
			log.Printf("Failed to register service %s: %v", svc.id, err)
		}
	}

	// 创建并注册自定义 resolver
	customBuilder := newCustomEtcdResolverBuilder(etcdClient)
	resolver.Register(customBuilder)

	// 创建 gRPC 连接
	conn, err := grpc.NewClient(
		fmt.Sprintf("%s:///%s", customScheme, serviceKey),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// 发起 RPC 调用
	makeRPCs(conn, 5)
}
