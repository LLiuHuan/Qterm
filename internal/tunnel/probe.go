package tunnel

import (
	"bytes"
	"context"
	"errors"
	"net"

	"golang.org/x/crypto/ssh"
)

var errHostKeyProbeComplete = errors.New("SSH host key probe complete")

type ProbeCollector func(hopIndex int, label string, address string, key ssh.PublicKey, known bool) error

func (c *Connector) ProbeHostKeys(
	ctx context.Context,
	legs []Hop,
	target Target,
	verify ssh.HostKeyCallback,
	collect ProbeCollector,
) error {
	if ctx == nil {
		return &ConnectError{Stage: StageValidation, Err: errors.New("SSH context is nil")}
	}
	if c == nil || c.dialContext == nil || c.timeout <= 0 {
		return &ConnectError{Stage: StageValidation, Err: ErrInvalidTarget}
	}
	if verify == nil {
		return &ConnectError{Stage: StageValidation, Err: ErrHostKeyVerifierRequired}
	}
	if collect == nil {
		return &ConnectError{Stage: StageValidation, Err: errors.New("SSH host key collector is nil")}
	}
	dial := c.dialContext
	relays := make([]*Connection, 0, len(legs))
	defer func() {

		for index := len(relays) - 1; index >= 0; index-- {
			_ = relays[index].Close()
		}
	}()

	accepted := make(map[string]ssh.PublicKey, len(legs))
	total := len(legs)
	for index := 0; index <= total; index++ {
		segment := target
		label := ""
		hopIndex := 0
		if index < total {
			segment = legs[index].Target
			label = legs[index].Label
			hopIndex = index + 1
		}
		address, _, err := normalizeTarget(segment)
		if err != nil {
			return &ConnectError{Stage: StageValidation, Err: err}
		}
		key, known, err := c.probeHostKey(ctx, segment, hopIndex, label, dial, verify)
		if err != nil {
			return probeSegmentFailure(err, index, total)
		}
		if err := collect(hopIndex, label, address, key, known); err != nil {
			return err
		}
		if index == total {
			break
		}
		if !known {
			accepted[address] = key
		}
		relay, err := c.probeRelay(ctx, legs[index], dial, accepted, verify)
		if err != nil {
			return HopConnectError(err, index)
		}
		relays = append(relays, relay)
		dial = relay.DialContext
	}
	return nil
}

func (c *Connector) probeHostKey(
	ctx context.Context,
	target Target,
	hopIndex int,
	label string,
	dial DialerFunc,
	verify ssh.HostKeyCallback,
) (ssh.PublicKey, bool, error) {
	address, username, err := normalizeTarget(target)
	if err != nil {
		return nil, false, &ConnectError{Stage: StageValidation, Err: err}
	}
	timeout := c.timeout
	if target.Timeout > 0 {
		timeout = target.Timeout
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	netConn, err := dial(probeCtx, "tcp", address)
	if err != nil {
		return nil, false, connectError(probeCtx, StageTCP, err)
	}
	defer func() { _ = netConn.Close() }()
	if deadline, ok := probeCtx.Deadline(); ok {

		_ = netConn.SetDeadline(deadline)
	}
	stopCancellation := context.AfterFunc(probeCtx, func() { _ = netConn.Close() })
	defer stopCancellation()

	var observedKey ssh.PublicKey
	known := false
	var verifyFailure error
	config := &ssh.ClientConfig{

		User: username,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			observedKey = key
			verifyErr := verify(hostname, remote, key)
			if verifyErr == nil {
				known = true
				return errHostKeyProbeComplete
			}
			var hostKeyErr *HostKeyError
			if errors.As(verifyErr, &hostKeyErr) {

				hostKeyErr.Challenge.HopIndex = hopIndex
				hostKeyErr.Challenge.HopLabel = label
				if hostKeyErr.Challenge.Issue == HostKeyIssueChanged {
					verifyFailure = hostKeyErr

					return hostKeyErr
				}

				return errHostKeyProbeComplete
			}
			verifyFailure = verifyErr
			return verifyErr
		},

		HostKeyAlgorithms: hostKeyAlgorithms,
	}
	_, _, _, handshakeErr := ssh.NewClientConn(netConn, address, config)
	if verifyFailure != nil {
		return nil, false, verifyFailure
	}

	if observedKey != nil {
		return observedKey, known, nil
	}
	return nil, false, connectError(probeCtx, StageHandshake, handshakeErr)
}

func (c *Connector) probeRelay(
	ctx context.Context,
	leg Hop,
	dial DialerFunc,
	accepted map[string]ssh.PublicKey,
	verify ssh.HostKeyCallback,
) (*Connection, error) {
	callback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if expected, ok := accepted[hostname]; ok && publicKeysEqual(expected, key) {
			return nil
		}
		return verify(hostname, remote, key)
	}

	relay := &Connector{dialContext: c.dialContext, hostKey: callback, timeout: c.timeout}
	return relay.connectOver(ctx, leg.Target, leg.Auth, dial)
}

func publicKeysEqual(left, right ssh.PublicKey) bool {
	if left == nil || right == nil {
		return false
	}
	return bytes.Equal(left.Marshal(), right.Marshal())
}

func probeSegmentFailure(err error, index, total int) error {
	if index < total {
		return HopConnectError(err, index)
	}
	var connectErr *ConnectError
	if total > 0 && errors.As(err, &connectErr) &&
		connectErr.Stage == StageTCP && IsForwardDenied(err) {
		return &ConnectError{Stage: StageJumpDial, Hop: total - 1, Err: err}
	}
	return err
}
