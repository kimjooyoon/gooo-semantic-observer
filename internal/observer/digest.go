package observer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func DigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func DigestJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "sha256:invalid"
	}
	return DigestBytes(raw)
}
