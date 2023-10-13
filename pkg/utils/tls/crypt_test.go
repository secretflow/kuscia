/*Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenCrypt(t *testing.T) {
	priKey, err := rsa.GenerateKey(rand.Reader, 2048)

	pub := EncodePKCS1PublicKey(priKey)

	pubKey, err := ParsePKCS1PublicKey(pub)
	assert.NoError(t, err)

	originToken := make([]byte, 16)
	_, err = rand.Read(originToken)
	text, err := EncryptPKCS1v15(pubKey, originToken)
	assert.NoError(t, err)

	decodedToken, err := DecryptPKCS1v15(priKey, text, 16)
	assert.NoError(t, err)
	assert.Equal(t, originToken, decodedToken)
}
