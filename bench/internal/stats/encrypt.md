# Encryption

The package provides envelope encryption for command payloads stored in the stats database.

# Types

# Functions

## LoadKey() ([]byte, error)

1. Load the key from the configured key path.

## loadKeyFrom(path) ([]byte, error)

1. Read the key file, returning nil when it does not exist.
2. Trim whitespace and return nil when nothing remains, so stats keep running without full command storage.
3. Base64-decode the contents.
4. Reject a key that is not 32 bytes.
5. Return the decoded key.

#### Errors

- **1.** if the file cannot be read for a reason other than not existing, return an error.

---

- **3.** if the contents are not valid base64, return an error; the caller disables stats.
- **4.** if the decoded key is not 32 bytes, return an error; the caller disables stats.

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
