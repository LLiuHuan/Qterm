package store

type EncryptedPayloadCodec interface {
	Seal(domain, id string, plaintext []byte) ([]byte, error)
	Open(domain, id string, envelope []byte) ([]byte, error)
}
