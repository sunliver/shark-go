package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"

	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/scrypt"
)

const (
	constKeySize = 256
)

type Crypto struct {
	block cipher.Block
	iv    []byte
}

func NewCrypto(password []byte) *Crypto {
	cipher, iv := aesHelper(password)
	return &Crypto{
		block: cipher,
		iv:    iv,
	}
}

func (c *Crypto) CryptBlocks(src []byte) []byte {
	paddingSzB := aes.BlockSize - len(src)%aes.BlockSize
	paddingBytes := bytes.Repeat([]byte{byte(paddingSzB)}, paddingSzB)
	dst := append(src, paddingBytes...)
	cipher.NewCBCEncrypter(c.block, c.iv).CryptBlocks(dst, dst)
	return dst
}

func (c *Crypto) DecrypBlocks(dst []byte) []byte {
	cipher.NewCBCDecrypter(c.block, c.iv).CryptBlocks(dst, dst)
	return dst[:len(dst)-int(dst[len(dst)-1])]
}

// aesHelper returns aes cbc block mode
// see https://golang.org/src/crypto/cipher/example_test.go for more information
func aesHelper(password []byte) (cipher.Block, []byte) {
	iv, err := scrypt.Key(password, password, 256, 8, 16, 16)
	if err != nil {
		panic(err)
	}

	key, err := scrypt.Key(password, iv, 512, 8, 16, 32)
	if err != nil {
		panic(err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	return block, iv
}

// ScryptHelper returns random data build from scrypt
func ScryptHelper() []byte {
	data, err := scrypt.Key(uuid.NewV4().Bytes(), uuid.NewV4().Bytes(), 1024, 8, 8, 16)
	if err != nil {
		panic(err)
	}
	return data
}
