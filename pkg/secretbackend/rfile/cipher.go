// Copyright 2023 Ant Group Co., Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rfile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/secretflow/kuscia/pkg/utils/nlog"
)

// Cipher will encrypt the data before writing it to the file and decrypt it after reading the data from the file.
type Cipher interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

const (
	aesKeySize        = 32
	aesKeyPaddingByte = 0x66
)

type AESCipherConfig struct {
	key string
}

type AESCipher struct {
	keyStr string
	key    []byte
	block  cipher.Block
}

func NewAESCipher(keyStr string) (Cipher, error) {
	key := []byte(keyStr)
	if len(key) < aesKeySize {
		nlog.Warnf("for aes, key size less than 256 bits, padding 0x66.")
		key = append(key, byteTimes(aesKeyPaddingByte, aesKeySize-len(key))...)
	}
	if len(key) > aesKeySize {
		nlog.Warnf("for aes, key size greater than 256 bits, cut off")
		key = key[:aesKeySize]
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &AESCipher{
		keyStr: keyStr,
		key:    key,
		block:  block,
	}, nil
}

func (c *AESCipher) Encrypt(value []byte) ([]byte, error) {

	plaintext := bytesClone(value)
	if len(plaintext)%c.block.BlockSize() != 0 {
		plaintext = pkcs7Padding(value, c.block.BlockSize())
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	// The IV needs to be unique, but not secure. Therefore, it's common to
	// include it at the beginning of the ciphertext.
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(c.block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

func (c *AESCipher) Decrypt(value []byte) ([]byte, error) {

	ciphertext := bytesClone(value)
	if len(ciphertext) < c.block.BlockSize() {
		return nil, fmt.Errorf("ciphertext invalid: too short")
	}

	// extract IV
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(c.block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	return pkcs7UnPadding(plaintext), nil
}

func pkcs7Padding(v []byte, blockSize int) []byte {
	paddingSize := blockSize - len(v)%blockSize
	// paddingSize is equals paddingContent
	padding := byteTimes(byte(paddingSize), paddingSize)
	return append(v, padding...)
}

func pkcs7UnPadding(v []byte) []byte {
	paddingSize := int(v[len(v)-1])
	return v[:(len(v) - paddingSize)]
}

func byteTimes(b byte, times int) []byte {
	collection := make([]byte, times)
	for i := 0; i < times; i++ {
		collection[i] = b
	}
	return collection
}

func bytesClone(bs []byte) []byte {
	clone := make([]byte, len(bs))
	for i, b := range bs {
		clone[i] = b
	}
	return clone
}
