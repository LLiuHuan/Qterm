package tunnel

import (
	"context"
	"errors"

	"golang.org/x/crypto/ssh"
)

var ErrSSHAgentUnavailable = errors.New("SSH agent is unavailable")

func (c *Connector) ConnectSSHAgent(ctx context.Context, target Target, publicKey []byte) (*Connection, error) {
	return c.ConnectSSHAgentAuthMethods(ctx, target, publicKey, nil, false)
}

func (c *Connector) ConnectSSHAgentAuthMethods(
	ctx context.Context,
	target Target,
	publicKey []byte,
	extra []ssh.AuthMethod,
	interactive bool,
) (*Connection, error) {
	agentConn, signer, err := openAgentSigner(ctx, publicKey)
	if err != nil {
		return nil, err
	}
	defer agentConn.Close()
	authMethods := append([]ssh.AuthMethod{ssh.PublicKeys(signer)}, extra...)
	return c.ConnectAuthMethods(ctx, target, authMethods, interactive)
}
