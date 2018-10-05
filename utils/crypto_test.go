package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrypto(t *testing.T) {
	passwd := []byte{0x12, 0x34, 0x56, 0x78}

	crypto := NewCrypto(passwd)

	texts := []string{
		"Hello Crypt",
		"",
		"Hello\r\n",
		"moremoremoremoremoremoremoremoremoremoremore",
	}

	for _, text := range texts {
		crypted := crypto.CryptBlocks([]byte(text))
		decrypted := crypto.DecrypBlocks(crypted)
		assert.Equal(t, text, string(decrypted))
	}
}
