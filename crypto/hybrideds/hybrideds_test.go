package hybrideds

import (
	"bytes"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"math/rand"
	"testing"
)

var (
	testmsg1 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626262")
	testmsg2 = hexutil.MustDecode("0x68692074686572656f636b636861696e62626262626262626262626262626261")
)

func TestHybrideds_Basic(t *testing.T) {
	if CRYPTO_SIGNATURE_BYTES != 2+64+2420+40+CRYPTO_MESSAGE_LEN {
		t.Fatal("incorrect sig size")
	}
	pubKey, priKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	priBytes, pubBytes, err := PrivateAndPublicFromPrivateKey(priKey)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(priKey, priBytes) != 0 {
		t.Fatal("PrivateAndPublicFromPrivateKey private compare failed")
	}

	if bytes.Compare(pubKey, pubBytes) != 0 {
		t.Fatal("PrivateAndPublicFromPrivateKey public compare failed")
	}

	digestHash1 := []byte(testmsg1)

	signature, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}

	err = Verify(digestHash1, signature, pubKey)
	if err != nil {
		t.Fatal(err)
	}

	digestHash1[0] = digestHash1[0] + 1
	err = Verify(digestHash1, signature, pubKey)
	if err == nil {
		t.Fatal(err)
	}

	signature2, err := Sign(priKey, digestHash1)
	if err != nil {
		t.Fatal(err)
	}
	err = Verify(digestHash1, signature2, pubKey)
	if err != nil {
		t.Fatal(err)
	}

}

func TestHybrideds_Random(t *testing.T) {

	var keyMap map[string]bool
	keyMap = make(map[string]bool)

	for i := 1; i < 100; i++ {
		pubKey, priKey, err := GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		pubKeyText := string(pubKey[:])
		if keyMap[pubKeyText] == true {
			t.Fatal("same key")
		}

		keyMap[pubKeyText] = true

		digestHash1 := make([]byte, 32)
		rand.Read(digestHash1)

		signature1, err := Sign(priKey, digestHash1)
		if err != nil {
			t.Fatal(err)
		}

		err = Verify(digestHash1, signature1, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		digestHash2 := make([]byte, 32)
		rand.Read(digestHash2)

		signature2, err := Sign(priKey, digestHash2)
		if err != nil {
			t.Fatal(err)
		}

		err = Verify(digestHash2, signature2, pubKey)
		if err != nil {
			t.Fatal(err)
		}

		err = Verify(digestHash2, signature1, pubKey)
		if err == nil {
			t.Fatal("verify passed while it should have failed")
		}

		err = Verify(digestHash1, signature2, pubKey)
		if err == nil {
			t.Fatal("verify passed while it should have failed")
		}
	}

}
