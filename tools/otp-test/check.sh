#!/bin/sh
set -eu

cd "$(dirname "$0")"

printf '%s\n' 'Checking the SSH service...'
docker compose -f compose.yaml exec -T otp-server sshd -T >/dev/null

printf '%s\n' 'Checking that an incorrect password is rejected...'
if ./ssh-auth.exp wrong-password; then
    printf '%s\n' 'PASS: incorrect password rejected by SSH'
else
    printf '%s\n' 'FAIL: incorrect password was accepted or the challenge did not complete' >&2
    exit 1
fi

printf '%s\n' 'Checking that an incorrect OTP is rejected...'
if ./ssh-auth.exp wrong-otp; then
    printf '%s\n' 'PASS: incorrect OTP rejected by SSH'
else
    printf '%s\n' 'FAIL: incorrect OTP was accepted or the challenge did not complete' >&2
    exit 1
fi

printf '%s\n' 'Checking that the current OTP is accepted...'
if ./ssh-auth.exp correct; then
    printf '%s\n' 'PASS: password plus current OTP accepted'
else
    printf '%s\n' 'FAIL: current OTP was rejected' >&2
    exit 1
fi
