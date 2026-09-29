# Encryption

The package provides envelope encryption for command payloads stored in the stats database.

# Types

# Functions

## LoadKey() ([]byte, error)

1. Read the key file, returning nil when it does not exist.
2. Base64-decode the contents.
3. Reject a key that is not 32 bytes.
4. Return the decoded key.

## Encrypt(key, plaintext) (string, error)

1. Build the cipher from the key.
2. Generate a fresh random nonce.
3. Seal the plaintext with GCM.
4. Return the envelope v1, base64 nonce, base64 ciphertext plus tag.

## Decrypt(key, envelope) (string, error)

1. Split the envelope and reject an unrecognized format.
2. Decode the nonce and ciphertext.
3. Open the ciphertext, verifying the tag.
4. Return the plaintext.

#### Rationale

- The envelope version prefix lets the tool reject envelopes from a different scheme.
- A fresh random nonce per seal prevents replay and ciphertext reuse.
