package etcdv3

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/DavadDi/grpclb/common"
	"google.golang.org/grpc"
	pb "google.golang.org/grpc/examples/helloworld/helloworld"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/resolver"
)

func TestEtcdResolver(t *testing.T) {
	etcdAddrs := []string{"127.0.0.1:2379"}
	srvName := "test"
	srvVersion := "v1"
	ttl := int64(10)

	register := NewEtcdRegister(etcdAddrs)

	log.Println("NewEtcdRegister")
	info := common.ServerNodeInfo{
		Name:           srvName,
		Version:        srvVersion,
		Addr:           "127.0.0.1:8080",
		Weight:         1,
		LastUpdateTime: time.Now(),
	}

	register.Register(info, ttl)

	resolver := NewEtcdResolver(etcdAddrs, srvName, srvVersion, ttl)
	resolver.JustForTest()

	_, err := resolver.Start()
	if err != nil {
		t.Fatalf("Start Resolver failed")
	}

	time.Sleep(2 * time.Second)

	register.Stop()

	info.Addr = "127.0.0.1:8090"
	register.Register(info, ttl)
	time.Sleep(2 * time.Second)
	register.Stop()

	time.Sleep(20 * time.Second)
}

// server is used to implement helloworld.GreeterServer.
type server struct {
	Port int
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Message: fmt.Sprintf("Hello From %d", s.Port)}, nil
}

var etcdAddrs = []string{"127.0.0.1:2379"}

var ss []*grpc.Server

func TestGRPCDialSignal(t *testing.T) {
	ss = []*grpc.Server{}

	name := "test"
	version := "v1"
	ttl := int64(10)

	r := NewEtcdResolver(etcdAddrs, name, version, ttl)
	resolver.Register(r)

	go newGrpcServer(t, name, version, 8089)

	addr := fmt.Sprintf("%s:///%s", r.Scheme(), name)
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBalancerName("round_robin"))
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}

	log.Println("Conect to " + addr + " succeed!")
	defer conn.Close()

	c := pb.NewGreeterClient(conn)

	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := c.SayHello(ctx, &pb.HelloRequest{Name: name})
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}

		log.Printf("Greeting: %s", resp.Message)

		time.Sleep(2 * time.Second)
	}

	log.Println("Ready to close server")
	for _, s := range ss {
		s.GracefulStop()
	}

}

func TestGRPCDial(t *testing.T) {
	ss = []*grpc.Server{}

	name := "test"
	version := "v1"
	ttl := int64(10)

	r := NewEtcdResolver(etcdAddrs, name, version, ttl)
	resolver.Register(r)

	go newGrpcServer(t, name, version, 9091)
	go newGrpcServer(t, name, version, 9092)

	addr := fmt.Sprintf("%s:///%s", r.Scheme(), name)
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBalancerName("round_robin"))
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}

	log.Println("Conect to " + addr + " succeed!")
	defer conn.Close()

	c := pb.NewGreeterClient(conn)

	go func() {
		time.Sleep(3 * time.Second)
		go newGrpcServer(t, name, version, 9093)

		time.Sleep(3 * time.Second)
		go newGrpcServer(t, name, version, 9094)
	}()

	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		resp, err := c.SayHello(ctx, &pb.HelloRequest{Name: name})
		if err != nil {
			log.Fatalf("could not greet: %v", err)
		}

		log.Printf("Greeting: %s", resp.Message)

		time.Sleep(500 * time.Millisecond)
	}

	for _, s := range ss {
		s.GracefulStop()
	}
}

func newGrpcServer(t *testing.T, name, version string, port int) {
	r := EtcdRegister{
		EtcdAddrs:   etcdAddrs,
		DialTimeout: 3,
	}

	defer r.Stop()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	log.Printf("listen at %d succeed\n", port)

	// Create a server
	s := grpc.NewServer()
	pb.RegisterGreeterServer(s, &server{Port: port})

	info := common.ServerNodeInfo{
		Name:           name,
		Version:        version,
		Addr:           fmt.Sprintf("127.0.0.1:%d", port),
		Weight:         1,
		LastUpdateTime: time.Now(),
	}

	r.Register(info, 10)
	ss = append(ss, s)

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		t.Fatalf("failed to serve: %v", err)
	}
}

// Only for test
func (r *EtcdResolver) Start() (chan<- struct{}, error) {
	return r.start()
}

// Stop Resover related process, GRPC will call Close() !!! Don't call Stop
func (r *EtcdResolver) Stop() {
	r.Close()
}

// Function Start and Stop just for local test
// JustForTest when Build() hasn't called
func (r *EtcdResolver) JustForTest() {
	r.usedForTest = true
}
