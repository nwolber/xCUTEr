package job

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want <%T>%+v, got: <%T>%+v", want, want, got, got)
	}
}

func TestMain(m *testing.M) {
	InitializeSSHClientStore(time.Hour * 24 * 365)

	retCode := m.Run()
	os.Exit(retCode)
}

func TestCreateClient(t *testing.T) {
	const (
		user     = "testUser"
		question = "Password: "
		answer   = "rootroot"
	)

	config := &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(c ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			a, err := client(user, "", []string{question}, []bool{false})
			expect(t, 1, len(a))
			expect(t, answer, a[0])
			expect(t, nil, err)

			if a[0] != answer {
				return nil, errors.New("invalid answer")
			}

			return nil, nil
		},
	}

	key, err := generateSSHKey()
	if err != nil {
		t.Fatal(err)
	}

	config.AddHostKey(key)

	s, err := newSSHTestServer(config, 0)
	if err != nil {
		t.Fatal("expected nil, got", err)
	}
	defer s.Close()

	client, err := createClient(s.listener.Addr().String(), user, "", "", map[string]string{
		question: answer,
	})
	if client == nil {
		t.Error("expected a client, got nil")
	}
	expect(t, nil, err)
}

type testServer struct {
	successfulConnections int32
	listener              net.Listener
	cancel                context.CancelFunc
}

func (s *testServer) Close() {
	s.cancel()
	s.listener.Close()
}

func newSSHTestServer(config *ssh.ServerConfig, sleepPeriod time.Duration) (*testServer, error) {
	s := &testServer{}
	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())

	s.listener = newLocalListener()

	go func() {
		conn, err := s.listener.Accept()

		if err != nil {
			// log.Println("sshtest: error accepting connection", err)
			return
		}

		select {
		case <-time.After(sleepPeriod):
		case <-ctx.Done():
		}

		_, chans, reqs, err := ssh.NewServerConn(conn, config)
		if err != nil {
			fmt.Println("error opening server conn", err)
			return
		}

		select {
		case <-time.After(sleepPeriod):
		case <-ctx.Done():
		}

		go ssh.DiscardRequests(reqs)

		go func() {
			for {
				select {
				case newChannel, ok := <-chans:
					if !ok {
						fmt.Println("new connection channel close")
						return
					}

					select {
					case <-time.After(sleepPeriod):
					case <-ctx.Done():
					}

					channel, _, err := newChannel.Accept()
					if err != nil {
						fmt.Println("error accepting new channel", err)
					}

					atomic.AddInt32(&s.successfulConnections, 1)

					select {
					case <-time.After(sleepPeriod):
					case <-ctx.Done():
					}
					channel.Close()
				case <-ctx.Done():
					return
				}
			}
		}()
	}()

	return s, nil
}

func generateSSHKey() (ssh.Signer, error) {
	pk, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return ssh.NewSignerFromSigner(pk)
}

// from net/http/httptest
func newLocalListener() net.Listener {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			panic(fmt.Sprintf("sstest: failed to listen on a port: %v", err))
		}
	}
	return l
}

func TestKIHappyPath(t *testing.T) {
	const (
		user     = "user"
		question = "Password: "
		pw       = "123"
	)
	c := keyboardInteractiveChallenge(user, map[string]string{
		question: pw,
	})
	answers, _ := c(user, "", []string{question}, nil)
	expect(t, 1, len(answers))
	expect(t, pw, answers[0])
}

func TestKINoQuestions(t *testing.T) {
	const (
		user     = "user"
		question = "Password: "
		pw       = "123"
	)
	c := keyboardInteractiveChallenge(user, map[string]string{
		question: pw,
	})
	answers, _ := c(user, "", []string{}, nil)
	expect(t, 0, len(answers))
}

func TestConnectionTimeout(t *testing.T) {
	conn := &errorConn{
		c:   make(chan struct{}),
		err: errors.New("errorConn closed"),
	}

	origCreateClient := createClient
	createClient = func(addr, user, keyFile, password string, keyboardInteractive map[string]string) (*sshClient, error) {
		return &sshClient{
			c:       &ssh.Client{Conn: conn},
			trashed: make(chan struct{}),
		}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client1, err := newSSHClient(ctx, "", "", "", "", nil)
	expect(t, nil, err)

	if client1 == nil {
		t.Fatalf("Expected client1 to be non-nil")
	}

	close(conn.c)
	<-client1.trashed

	client2, err := newSSHClient(ctx, "", "", "", "", nil)
	expect(t, nil, err)

	if client2 == nil {
		t.Fatalf("Expected client2 to be non-nil")
	}

	if client1 == client2 {
		t.Fatalf("Expected to get a new client, got the old one")
	}

	createClient = origCreateClient
}

type errorConn struct {
	c   chan struct{}
	err error
}

func (e *errorConn) User() string { return "no user" }

func (e *errorConn) SessionID() []byte { return []byte{} }

func (e *errorConn) ClientVersion() []byte { return []byte{} }

func (e *errorConn) ServerVersion() []byte { return []byte{} }

func (e *errorConn) RemoteAddr() net.Addr { panic("") }

func (e *errorConn) LocalAddr() net.Addr { panic("") }

func (e *errorConn) SendRequest(name string, wantReply bool, payload []byte) (bool, []byte, error) {
	return false, []byte{}, nil
}

func (e *errorConn) OpenChannel(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, errors.New("error")
}

func (e *errorConn) Close() error { return nil }

func (e *errorConn) Wait() error {
	<-e.c
	return e.err
}

func TestSlowServer(t *testing.T) {
	config := &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(c ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			return nil, nil
		},
	}

	key, err := generateSSHKey()
	if err != nil {
		t.Fatal(err)
	}

	config.AddHostKey(key)

	slowServer, err := newSSHTestServer(config, 5*time.Second)
	if err != nil {
		t.Fatal("failed to create slow server", err)
	}
	defer slowServer.cancel()
	t.Log("slow server at", slowServer.listener.Addr())

	fastServer, err := newSSHTestServer(config, 0)
	if err != nil {
		t.Fatal("failed to create fast server", err)
	}
	defer fastServer.cancel()
	t.Log("fast server at", fastServer.listener.Addr())

	responses := make(chan string)
	slowStarted := make(chan struct{})
	ctx := context.Background()

	go func() {
		slowStarted <- struct{}{}
		slowClient, err := newSSHClient(ctx, slowServer.listener.Addr().String(), "user", "", "", map[string]string{"question": "answer"})
		if err != nil {
			responses <- fmt.Sprint("failed to create slow client", err)
			return
		}
		defer slowClient.c.Close()
		responses <- "slow"
	}()

	go func() {
		<-slowStarted
		fastClient, err := newSSHClient(ctx, fastServer.listener.Addr().String(), "user", "", "", map[string]string{"question": "answer"})
		if err != nil {
			responses <- fmt.Sprint("failed to create fast client", err)
			return
		}
		defer fastClient.c.Close()
		responses <- "fast"
	}()

	expect(t, "fast", <-responses)
}
