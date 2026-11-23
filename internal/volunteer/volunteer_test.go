package volunteer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/colmeabhi/sparecycles/gen/sparecycles/v1"
	"github.com/colmeabhi/sparecycles/internal/leaderclient"
	"github.com/colmeabhi/sparecycles/internal/security"
	"github.com/colmeabhi/sparecycles/internal/workload"
)

// fakeLeader rejects the session of the first registration once, then accepts
// the session of the next registration.
type fakeLeader struct {
	pb.UnimplementedVolunteerServiceServer

	mu        sync.Mutex
	secrets   [][]byte    // secret of each registration, in order
	accepted  chan []byte // secrets that RequestWork accepted
	rejectOne bool        // true until the first rejection
}

func (f *fakeLeader) Register(context.Context, *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	secret, _ := security.NewSession()
	f.secrets = append(f.secrets, secret)
	return &pb.RegisterResponse{VolunteerId: fmt.Sprintf("vol-%d", len(f.secrets)), SessionSecret: secret}, nil
}

func (f *fakeLeader) RequestWork(_ context.Context, req *pb.RequestWorkRequest) (*pb.RequestWorkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rejectOne {
		f.rejectOne = false
		return nil, status.Error(codes.Unauthenticated, security.ErrBadSession.Error())
	}
	f.accepted <- req.GetSessionSecret()
	return &pb.RequestWorkResponse{RetryAfterMs: 1000}, nil
}

func (f *fakeLeader) registrations() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.secrets
}

func TestVolunteerRegistersAgainAfterRejectedSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	leader := &fakeLeader{accepted: make(chan []byte, 10), rejectOne: true}
	lis := bufconn.Listen(1 << 20)
	g := grpc.NewServer()
	pb.RegisterVolunteerServiceServer(g, leader)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	v := New(Config{Name: "v"}, leaderclient.FromConns(conn), workload.Default(), security.NoSign{}, quiet)
	go func() { _ = v.Run(ctx) }()

	select {
	case got := <-leader.accepted:
		secrets := leader.registrations()
		if len(secrets) != 2 {
			t.Fatalf("registrations = %d, want 2 (one before and one after the rejection)", len(secrets))
		}
		if !bytes.Equal(got, secrets[1]) {
			t.Fatal("after registering again, the volunteer did not send the new secret")
		}
	case <-ctx.Done():
		t.Fatal("the volunteer did not register again after the rejected session")
	}
}
