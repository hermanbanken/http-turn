from hashlib import sha1
import hmac

key = b"foobar" 

# The Base String as specified here: 
raw = b"1599491771" # as specified by OAuth

hashed = hmac.new(key, raw, sha1)

# The signature
print(hashed.digest().encode("base64").rstrip('\n'))
