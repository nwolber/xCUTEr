package job

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"

	"golang.org/x/crypto/ssh"
)

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want %q, got: %q", want, got)
	}
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

	s, err := newSSHTestServer(config)
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

type testServer struct {
	successfulConnections int32
	listener              net.Listener
	cancel                context.CancelFunc
}

func (s *testServer) Close() {
	s.cancel()
	s.listener.Close()
}

func newSSHTestServer(config *ssh.ServerConfig) (*testServer, error) {
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

		_, chans, reqs, err := ssh.NewServerConn(conn, config)
		if err != nil {
			fmt.Println("error opening server conn", err)
			return
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
					channel, _, err := newChannel.Accept()
					if err != nil {
						fmt.Println("error accepting new channel", err)
					}

					atomic.AddInt32(&s.successfulConnections, 1)
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
