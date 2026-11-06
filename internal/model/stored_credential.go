package model

type StoredCredential struct {
	Metadata Credential
	Value    []byte
}
