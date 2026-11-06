#!/usr/bin/env python3
"""Generate the public test fixture's RFC 6238 TOTP without external packages."""

import base64
import hashlib
import hmac
import struct
import sys
import time


SECRET = "JBSWY3DPEHPK3PXP"


def code(offset: int = 0) -> str:
    key = base64.b32decode(SECRET + "=" * ((8 - len(SECRET) % 8) % 8))
    counter = int(time.time() // 30) + offset
    digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
    start = digest[-1] & 0x0F
    value = struct.unpack(">I", digest[start : start + 4])[0] & 0x7FFFFFFF
    result = value % 1_000_000
    return f"{result:06d}"


if __name__ == "__main__":
    current = code()
    if len(sys.argv) > 1 and sys.argv[1] == "wrong":
        current = current[:-1] + str((int(current[-1]) + 1) % 10)
    print(current)
