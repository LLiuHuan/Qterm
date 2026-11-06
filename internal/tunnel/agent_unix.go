//go:build !windows

package tunnel

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func openAgentSigner(ctx context.Context, publicKey []byte) (net.Conn, ssh.Signer, error) {
	socket := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
	if socket == "" {
		return nil, nil, ErrSSHAgentUnavailable
	}
	key, _, _, rest, err := ssh.ParseAuthorizedKey(bytes.TrimSpace(publicKey))
	if err != nil || key == nil || len(bytes.TrimSpace(rest)) != 0 {
		return nil, nil, errors.New("invalid SSH agent public key")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, nil, ErrSSHAgentUnavailable
	}
	client := agent.NewClient(conn)
	signers, err := client.Signers()
	if err != nil {
		_ = conn.Close()
		return nil, nil, ErrSSHAgentUnavailable
	}
	for _, signer := range signers {
		if bytes.Equal(signer.PublicKey().Marshal(), key.Marshal()) {
			return conn, signer, nil
		}
	}
	_ = conn.Close()
	return nil, nil, errors.New("SSH agent key is not loaded")
}
