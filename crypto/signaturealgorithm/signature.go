package signaturealgorithm

import (
	"github.com/QuantumCoinProject/qc/common"
	"math/big"
)

type PublicKey struct {
	PubData []byte
}

type PrivateKey struct {
	PublicKey // public part.
	PriData   []byte
}

type SignatureAlgorithm interface {
	SignatureName() string
	PublicKeyLength() int
	PrivateKeyLength() int
	SignatureLength() int
	SignatureWithPublicKeyLength() int
	PublicKeyStartValue() byte
	SignatureStartValue() byte

	GenerateKey() (*PrivateKey, error)

	SerializePrivateKey(*PrivateKey) ([]byte, error)
	DeserializePrivateKey([]byte) (*PrivateKey, error)

	SerializePublicKey(*PublicKey) ([]byte, error)
	DeserializePublicKey([]byte) (*PublicKey, error)

	HexToPrivateKey(string) (*PrivateKey, error)
	HexToPrivateKeyNoError(string) *PrivateKey

	PrivateKeyToHex(*PrivateKey) (string, error)

	HexToPublicKey(string) (*PublicKey, error)
	PublicKeyToHex(*PublicKey) (string, error)

	LoadPrivateKeyFromFile(file string) (*PrivateKey, error)
	SavePrivateKeyToFile(file string, key *PrivateKey) error

	PublicKeyToAddress(*PublicKey) (common.Address, error)
	PublicKeyToAddressNoError(*PublicKey) common.Address

	EncodePublicKey(*PublicKey) []byte
	DecodePublicKey([]byte) (*PublicKey, error)

	Sign(digestHash []byte, prv *PrivateKey) (sig []byte, err error)
	Verify(pubKey []byte, digestHash []byte, signature []byte) bool

	SignWithContext(digestHash []byte, prv *PrivateKey, context []byte) (sig []byte, err error)
	VerifyWithContext(pubKey []byte, digestHash []byte, signature []byte, context []byte) bool

	Zeroize(prv *PrivateKey)

	PublicKeyAndSignatureFromCombinedSignature(digestHash []byte, sig []byte) (signature []byte, pubKey []byte, err error)

	CombinePublicKeySignature(sigBytes []byte, pubKeyBytes []byte) (combinedSignature []byte, err error)

	PublicKeyBytesFromSignature(digestHash []byte, sig []byte) ([]byte, error)

	PublicKeyFromSignature(digestHash []byte, sig []byte) (*PublicKey, error)
	PublicKeyFromSignatureWithContext(digestHash []byte, sig []byte, context []byte) (*PublicKey, error)

	ValidateSignatureValues(digestHash []byte, v byte, r, s *big.Int) bool
}
