import argparse
import base64
import hashlib

# All the code come from onionbalance codebase
# https://gitlab.torproject.org/tpo/core/onionbalance/-/blob/main/onionbalance/hs_v3/ext/slow_ed25519.py


def load_tor_key_from_disk(key_bytes):
    if key_bytes[:29] != b'== ed25519v1-secret: type0 ==':
        raise "Tor key does not start with Tor header"
    expanded_sk = key_bytes[32:]
    if len(expanded_sk) != 64:
        raise "Tor private key has the wrong length"
    return expanded_sk


def sign(priv_key, msg):
    return signatureWithESK(msg, priv_key, publickeyFromESK(priv_key))


def publickeyFromESK(h):
    a = decodeint(h[:32])
    A = scalarmult(B,a)
    return encodepoint(A)


def signatureWithESK(m,h,pk):
    a = decodeint(h[:32])
    tohint = b''.join([bytes([h[i]]) for i in range(b//8,b//4)]) + m
    r = Hint(tohint)
    R = scalarmult(B,r)
    S = (r + Hint(encodepoint(R) + pk + m) * a) % l
    return encodepoint(R) + encodeint(S)


def scalarmult(P,e):
    if e == 0: return [0,1]
    Q = scalarmult(P,e//2)
    Q = edwards(Q,Q)
    if e & 1: Q = edwards(Q,P)
    return Q


def inv(x):
    return expmod(x,q-2,q)


def expmod(b,e,m):
    if e == 0: return 1
    t = expmod(b,e//2,m)**2 % m
    if e & 1: t = (t*b) % m
    return t


q = 2**255 - 19
d = -121665 * inv(121666)
I = expmod(2,(q-1)//4,q)


def xrecover(y):
    xx = (y*y-1) * inv(d*y*y+1)
    x = expmod(xx,(q+3)//8,q)
    if (x*x - xx) % q != 0: x = (x*I) % q
    if x % 2 != 0: x = q-x
    return x


b = 256
l = 2**252 + 27742317777372353535851937790883648493
By = 4 * inv(5)
Bx = xrecover(By)
B = [Bx % q,By % q]

def edwards(P,Q):
    x1 = P[0]
    y1 = P[1]
    x2 = Q[0]
    y2 = Q[1]
    x3 = (x1*y2+x2*y1) * inv(1+d*x1*x2*y1*y2)
    y3 = (y1*y2+x1*x2) * inv(1-d*x1*x2*y1*y2)
    return [x3 % q,y3 % q]


def H(m):
    return hashlib.sha512(m).digest()


def bit(h,i):
    return (h[i//8] >> (i%8)) & 1


def Hint(m):
    h = H(m)
    return sum(2**i * bit(h,i) for i in range(2*b))


def encodepoint(P):
    x = P[0]
    y = P[1]
    bits = [(y >> i) & 1 for i in range(b - 1)] + [x & 1]
    return b''.join([bytes([sum([bits[i * 8 + j] << j for j in range(8)])]) for i in range(b//8)])


def encodeint(y):
    bits = [(y >> i) & 1 for i in range(b)]
    return b''.join([bytes([sum([bits[i * 8 + j] << j for j in range(8)])]) for i in range(b//8)])


def decodeint(s):
    return sum(2**i * bit(s,i) for i in range(0,b))


parser = argparse.ArgumentParser()
parser.add_argument('cert')
parser.add_argument('-s', '--secret', default="hs_ed25519_secret_key")
args = parser.parse_args()

msg = open(args.cert, 'rb').read()
pem_key_bytes = open(args.secret, 'rb').read()
privkey = load_tor_key_from_disk(pem_key_bytes)
msg_sig = sign(privkey, msg)
print(base64.b64encode(msg_sig).decode())