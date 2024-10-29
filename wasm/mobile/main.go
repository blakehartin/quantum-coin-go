package main

import "C"
import (
	"encoding/hex"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/common/hexutil"
	"github.com/QuantumCoinProject/qc/crypto"
	"github.com/QuantumCoinProject/qc/params"
	abi "github.com/QuantumCoinProject/qc/wasm/accounts/abi"
	wasm "github.com/QuantumCoinProject/qc/wasm/core/types"
	"golang.org/x/crypto/scrypt"
	"math/big"
	"strconv"
	"strings"
	"unsafe"
)

type Transaction struct {
	Transaction []TransactionDetails `json:"transaction"`
}

type TransactionDetails struct {
	FromAddress common.Address `json:"fromAddress"`
	ToAddress   common.Address `json:"toAddress"`
	Nonce       uint64         `json:"nonce"`
	GasLimit    uint64         `json:"gasLimit"`
	Value       *big.Int       `json:"value"`
	Data        []byte         `json:"data"`
	ChainId     *big.Int       `json:"chainId"`
}

func main() {

}

//export Scrypt
func Scrypt(skKeyStr, saltStr *C.char, skCount int) (*C.char, *C.char) {
	secret := C.GoBytes(unsafe.Pointer(skKeyStr), C.int(skCount))

	salt, err := base64.StdEncoding.DecodeString(C.GoString(saltStr))
	if err != nil {
		return nil, C.CString(err.Error())
	}

	derivedKey, err := scrypt.Key(secret, salt, 262144, 8, 1, 32)
	if err != nil {
		return nil, C.CString(err.Error())
	}
	return C.CString(base64.StdEncoding.EncodeToString(derivedKey)), nil
}

//export PublicKeyToAddress
func PublicKeyToAddress(pKeyStr *C.char, pkCount int) (*C.char, *C.char) {
	pubBytes := C.GoBytes(unsafe.Pointer(pKeyStr), C.int(pkCount))
	address := common.BytesToAddress(crypto.Keccak256(pubBytes[:])[common.AddressTruncateBytes:]).String()
	return C.CString(address), nil
}

//export IsValidAddress
func IsValidAddress(addressStr *C.char) (*C.char, *C.char) {
	address := C.GoString(addressStr)
	return C.CString(strconv.FormatBool(common.IsHexAddress(address))), nil
}

//export TxnSigningHash
func TxnSigningHash(from, nonce, to, value, gasLimit, data, chainId *C.char) (*C.char, *C.char) {
	ts, err := transaction(C.GoString(from), C.GoString(nonce), C.GoString(to),
		C.GoString(value), C.GoString(gasLimit), C.GoString(data), C.GoString(chainId))

	if err != nil {
		fmt.Println("TxnSigningHash err", err)
		return nil, C.CString(err.Error())
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	signerHash, err := signer.Hash(tx)
	if err != nil {
		return nil, C.CString(err.Error())
	}

	var message strings.Builder
	for i := 0; i < len(signerHash); i++ {
		sh := signerHash[i]
		message.WriteString(string(sh))
	}

	return C.CString(message.String()), nil
}

//export TxHash
func TxHash(from, nonce, to, value, gasLimit, data, chainId,
	pKeyStr, sigStr *C.char, pkCount int, sigCount int) (*C.char, *C.char) {

	ts, err := transaction(C.GoString(from), C.GoString(nonce), C.GoString(to),
		C.GoString(value), C.GoString(gasLimit), C.GoString(data), C.GoString(chainId))
	if err != nil {
		fmt.Println("TxHash err", err)
		return nil, C.CString(err.Error())
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubBytes := C.GoBytes(unsafe.Pointer(pKeyStr), C.int(pkCount))
	sigBytes := C.GoBytes(unsafe.Pointer(sigStr), C.int(sigCount))

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil, C.CString(err.Error())
	}

	return C.CString(signTx.Hash().String()), nil
}

//export TxData
func TxData(from, nonce, to, value, gasLimit, data, chainId,
	pKeyStr, sigStr *C.char, pkCount int, sigCount int) (*C.char, *C.char) {

	ts, err := transaction(C.GoString(from), C.GoString(nonce), C.GoString(to),
		C.GoString(value), C.GoString(gasLimit), C.GoString(data), C.GoString(chainId))
	if err != nil {
		fmt.Println("TxHash err", err)
		return nil, C.CString(err.Error())
	}

	tx := wasm.NewDefaultFeeTransaction(ts.Transaction[0].ChainId, ts.Transaction[0].Nonce,
		&ts.Transaction[0].ToAddress, ts.Transaction[0].Value,
		ts.Transaction[0].GasLimit, wasm.GAS_TIER_DEFAULT, ts.Transaction[0].Data)

	signer := wasm.NewLondonSigner(ts.Transaction[0].ChainId)

	pubBytes := C.GoBytes(unsafe.Pointer(pKeyStr), C.int(pkCount))
	sigBytes := C.GoBytes(unsafe.Pointer(sigStr), C.int(sigCount))

	signTx, err := signTxHash(tx, signer, pubBytes, sigBytes)
	if err != nil {
		return nil, C.CString(err.Error())
	}

	signTxBinary, err := signTx.MarshalBinary()
	if err != nil {
		return nil, C.CString(err.Error())
	}

	signTxEncode := hexutil.Encode(signTxBinary)
	return C.CString(signTxEncode), nil
}

//export ContractData
func ContractData(args **C.char, argvLength int) (*C.char, *C.char) {
	var method string
	var abiString string
		
	arguments := make([]interface{}, 0, argvLength-2)
	
	length := argvLength
	cStrings := (*[1 << 28]*C.char)(unsafe.Pointer(args))[:length:length]
	
	for i, cString := range cStrings { 
		fmt.Println("cString : ", cString)
	    switch i { 
			case 0: 
       			method = C.GoString(cString)
       		case 1: 
				abiString = C.GoString(cString)
	    	default:  
				arguments = append(arguments, C.GoString(cString))
	    }
	}

	abiData, err := abi.JSON(strings.NewReader(abiString))
	if err != nil {
		return nil, C.CString(err.Error())
	}
	
	data, err := abiData.Pack(method, arguments...)
	if err != nil {
		return nil, C.CString(err.Error())
	}

	d := hex.EncodeToString(data)

	return C.CString(d), nil
}

//export ParseBigFloat
func ParseBigFloat(value *C.char) (*C.char, *C.char) {
	f := new(big.Float)
	f.SetPrec(236)
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(C.GoString(value), f)
	if err != nil {
		return nil, C.CString(err.Error())
	}
	return C.CString(f.String()), nil
}

//export ParseBigFloatInner
func ParseBigFloatInner(value *C.char) (*C.char, *C.char) {
	f := new(big.Float)
	f.SetPrec(236) //  IEEE 754 octuple-precision binary floating-point format: binary256
	f.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(C.GoString(value), f)
	if err != nil {
		return nil, C.CString(err.Error())
	}
	return C.CString(f.String()), nil
}

//export WeiToEther
func WeiToEther(wei *C.char) (*C.char, *C.char) {
	val, _ := new(big.Int).SetString(C.GoString(wei), 10)
	return C.CString(new(big.Int).Div(val, big.NewInt(params.Ether)).String()), nil
}

//export EtherToWeiFloat
func EtherToWeiFloat(ethVal *C.char) (*C.char, *C.char) {
	val := C.GoString(ethVal)
	eth := new(big.Float)
	eth.SetPrec(236)
	eth.SetMode(big.ToNearestEven)
	_, err := fmt.Sscan(val, eth)
	if err != nil {
		return nil, C.CString(err.Error())
	}
	truncInt, _ := eth.Int(nil)
	truncInt = new(big.Int).Mul(truncInt, big.NewInt(params.Ether))
	fracStr := strings.Split(fmt.Sprintf("%.18f", eth), ".")[1]
	fracStr += strings.Repeat("0", 18-len(fracStr))
	fracInt, _ := new(big.Int).SetString(fracStr, 10)
	wei := new(big.Int).Add(truncInt, fracInt)
	return C.CString(wei.String()), nil
}

func transaction(args0, args1, args2, args3, args4, args5, args6 string) (transaction Transaction, e error) {
	var t Transaction
	var fromAddress = common.HexToAddress(args0)
	n, _ := strconv.Atoi(args1)
	var nonce = uint64(n)
	var toAddress = common.HexToAddress(args2)

	ethVal, err := ParseBigFloatInner(C.CString(args3))
	if err != nil {
		fmt.Println("ParseBigFloatInner", args3, "err", err)
		return t, errors.New(C.GoString(err))
	}

	wei, err := EtherToWeiFloat(ethVal)
	if err != nil {
		fmt.Println("EtherToWeiFloat", ethVal, "err", err)
		return t, errors.New(C.GoString(err))
	}

	weiVal, _ := new(big.Int).SetString(C.GoString(wei), 10)

	g, _ := strconv.Atoi(args4)
	var gasLimit = uint64(g)

	//var data []byte 
	data, _ := hex.DecodeString(args5)
	
	var chainId, _ = new(big.Int).SetString(args6, 0)
	
	transactionDetails := TransactionDetails{
		FromAddress: fromAddress, ToAddress: toAddress, Nonce: nonce, GasLimit: gasLimit,
		Value: weiVal, Data: data, ChainId: chainId}

	t.Transaction = append(t.Transaction, transactionDetails)

	return t, nil
}

func signTxHash(tx *wasm.Transaction, signer wasm.Signer, pubBytes, sigBytes []byte) (*wasm.Transaction, error) {
	sig := common.CombineTwoParts(sigBytes, pubBytes)
	return tx.WithSignature(signer, sig)
}
