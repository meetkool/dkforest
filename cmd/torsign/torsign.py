import argparse
import base64
import hashlib
from typing import NamedTuple

# All the code come from onionbalance codebase
# https://gitlab.torproject.org/tpo/core/onionbalance/-/blob/main/onionbalance/hs_v3/ext/slow_ed25519.py

def load_tor_key_from_disk(key_bytes: bytes) -> NamedTuple('TorKey', [('expanded_sk', bytes)]):
    """Load a Tor private key from disk and validate it.

    Returns a named tuple with the expanded secret key.
    """
    if key_bytes[:29] != b'== ed25519v1-secret: type0 ==':
        raise ValueError("Tor key does not start with Tor header")
    expanded_sk = key_bytes[32:]
    if len(expanded_sk) != 64:
        raise ValueError("Tor private key has the wrong length")
    return NamedTuple('TorKey', [('expanded_sk', bytes)])(expanded_sk)

# ... (rest of the functions here)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('cert')
    parser.add_argument('-s', '--secret', default="hs_ed25519_secret_key")
    args = parser.parse_args()

    try:
        with open(args.cert, 'rb') as f:
            msg = f.read()
    except FileNotFoundError:
        print(f"Error: Could not open file '{args.cert}'")
        return

    try:
        with open(args.secret, 'rb') as f:
            pem_key_bytes = f.read()
    except FileNotFoundError:
        print(f"Error: Could not open file '{args.secret}'")
        return

    privkey = load_tor_key_from_disk(pem_key_bytes)
    msg_sig = sign(privkey.expanded_sk, msg)
    print(base64.b64encode(msg_sig).decode())

if __name__ == '__main__':
    main()
