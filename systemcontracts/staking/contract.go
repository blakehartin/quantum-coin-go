package staking

import (
	"fmt"
	"github.com/QuantumCoinProject/qc/accounts/abi"
	"github.com/QuantumCoinProject/qc/common"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking/stakingv1"
	"github.com/QuantumCoinProject/qc/systemcontracts/staking/stakingv2"
	"strings"
)

// Steps after Contract is modified
// 1) solc --bin --bin-runtime --abi c:\github\go-dp\systemcontracts\staking\stakingv2\StakingContract.sol  -o c:\github\go-dp\systemcontracts\staking\stakingv2
// 2) abigen --bin=c:\github\go-dp\systemcontracts\staking\stakingv2\StakingContract.bin --abi=c:\github\go-dp\systemcontracts\staking\stakingv2\StakingContract.abi --pkg=staking --out=c:\github\go-dp\systemcontracts\staking\stakingv2\staking.go
// 3) in staking\stakingv2\staking.go, change the package name to just "stakingv2" (instead of "staking") to clear the error
// 4) copy StakingContract-runtime.bin into stakingbin.go STAKING_RUNTIME_BIN field
const STAKING_CONTRACT = "0x0000000000000000000000000000000000000000000000000000000000001000"

var STAKING_CONTRACT_ADDRESS = common.HexToAddress(STAKING_CONTRACT)

const PROOF_OF_STAKE_STAKING_CONTRACT_BLOCK_NUMBER = 1

var (
	stakingContract = STAKING_CONTRACT

	systemContracts      []string
	SystemContractsData  = make(map[string]*Contracts)
	systemContractVerify map[common.Address]bool
)

type Contracts struct {
	ContractAddressString string    `json:"ContractAddressString"`
	Contracts             *Contract `json:"Contracts"`
}

type Contract struct {
	ContractAddress common.Address `json:"ContractAddress"`
	ABI             string         `json:"ABI"`
	BIN             string         `json:"BIN"`
	Methods         *Method        `json:"Methods"`
	CallerAddress   common.Address `json:"CallerAddress"`
}

type Method struct {
	Deposits   *Deposit   `json:"Deposits"`
	Validators *Validator `json:"Validators"`
}

type Deposit struct {
	NewDeposit               string `json:"newDeposit"`
	InitiateWithdrawal       string `json:"initiateWithdrawal"`
	CompleteWithdrawal       string `json:"completeWithdrawal"`
	GetDepositorCount        string `json:"getDepositorCount"`
	GetTotalDepositedBalance string `json:"getTotalDepositedBalance"`
	GetValidatorOfDepositor  string `json:"getValidatorOfDepositor"`
	GetDepositorRewards      string `json:"getDepositorRewards"`
	GetDepositorSlashings    string `json:"getDepositorSlashings"`
	GetWithdrawalBlock       string `json:"getWithdrawalBlock"`
	DoesDepositorExist       string `json:"doesDepositorExist"`
	DidDepositorEverExist    string `json:"didDepositorEverExist"`
}

type Validator struct {
	GetBalanceOfDepositor    string `json:"getBalanceOfDepositor"`
	ListValidators           string `json:"listValidators"`
	GetDepositorOfValidator  string `json:"getDepositorOfValidator"`
	GetNetBalanceOfDepositor string `json:"getNetBalanceOfDepositor"`
	AddDepositorSlashing     string `json:"addDepositorSlashing"`
	AddDepositorReward       string `json:"addDepositorReward"`
	IsValidationPaused       string `json:"isValidationPaused"`
	DoesValidatorExist       string `json:"doesValidatorExist"`
	DidValidatorEverExist    string `json:"didValidatorEverExist"`
}

var (
	methods_collection = &Method{
		Deposits: &Deposit{
			NewDeposit:               "newDeposit",
			InitiateWithdrawal:       "initiateWithdrawal",
			CompleteWithdrawal:       "completeWithdrawal",
			GetDepositorCount:        "getDepositorCount",
			GetTotalDepositedBalance: "getTotalDepositedBalance",
			GetValidatorOfDepositor:  "getValidatorOfDepositor",
			GetDepositorRewards:      "getDepositorRewards",
			GetDepositorSlashings:    "getDepositorSlashings",
			DoesDepositorExist:       "doesDepositorExist",
			DidDepositorEverExist:    "didDepositorEverExist",
			GetWithdrawalBlock:       "getWithdrawalBlock",
		},
		Validators: &Validator{
			GetBalanceOfDepositor:    "getBalanceOfDepositor",
			ListValidators:           "listValidators",
			GetDepositorOfValidator:  "getDepositorOfValidator",
			GetNetBalanceOfDepositor: "getNetBalanceOfDepositor",
			AddDepositorSlashing:     "addDepositorSlashing",
			AddDepositorReward:       "addDepositorReward",
			IsValidationPaused:       "isValidationPaused",
			DoesValidatorExist:       "doesValidatorExist",
			DidValidatorEverExist:    "didValidatorEverExist",
		},
	}
)

func init() {
	if len(systemContracts) > 0 {
		return
	}
	systemContracts = []string{
		stakingContract,
	}

	SystemContractsData[stakingContract] = &Contracts{
		ContractAddressString: stakingContract,
		Contracts: &Contract{
			ContractAddress: common.HexToAddress(stakingContract),
			Methods:         methods_collection,
		},
	}

	systemContractVerify = map[common.Address]bool{
		common.HexToAddress(stakingContract): true,
	}
}

func GetContracts() []string {
	return systemContracts
}

func GetContract_Data(contract string) *Contract {
	return SystemContractsData[contract].Contracts
}

func GetContractVerify(address common.Address) bool {
	return systemContractVerify[address]
}

func IsStakingContract() error {
	if len(stakingContract) < (common.HashLength * 2) {
		return fmt.Errorf("Staking contractor is not found")
	}
	return nil
}

func GetStakingContract_Address_String() string {
	return SystemContractsData[stakingContract].ContractAddressString
}

func GetStakingContract_Address() common.Address {
	return SystemContractsData[stakingContract].Contracts.ContractAddress
}

func GetStakingContract_ABI() (abi.ABI, error) {
	s := stakingv1.StakingMetaData.ABI
	abi, err := abi.JSON(strings.NewReader(s))
	return abi, err
}

func GetStakingContractV2_ABI() (abi.ABI, error) {
	s := stakingv2.StakingMetaData.ABI
	abi, err := abi.JSON(strings.NewReader(s))
	return abi, err
}

func GetContract_Method_NewDeposit() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.NewDeposit
}

func GetContract_Method_InitiateWithdrawal() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.InitiateWithdrawal
}

func GetContract_Method_CompleteWithdrawal() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.CompleteWithdrawal
}

// Validators method
func GetContract_Method_ListValidators() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.ListValidators
}

func GetContract_Method_GetValidatorOfDepositor() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetValidatorOfDepositor
}

func GetContract_Method_GetDepositorOfValidator() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.GetDepositorOfValidator
}

func GetContract_Method_GetBalanceOfDepositor() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.GetBalanceOfDepositor
}

func GetContract_Method_IsValidationPaused() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.IsValidationPaused
}

func GetContract_Method_PauseValidation() string {
	return "pauseValidation"
}

func GetContract_Method_ResumeValidation() string {
	return "resumeValidation"
}

func GetContract_Method_GetNetBalanceOfDepositor() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.GetNetBalanceOfDepositor
}

func GetContract_Method_DoesValidatorExist() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.DoesValidatorExist
}

func GetContract_Method_DidValidatorEverExist() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.DidValidatorEverExist
}

func GetContract_Method_DoesDepositorExist() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.DoesDepositorExist
}

func GetContract_Method_DidDepositorEverExist() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.DidDepositorEverExist
}

func GetContract_Method_GetDepositorCount() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetDepositorCount
}

func GetContract_Method_GetTotalDepositedBalance() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetTotalDepositedBalance
}

func GetContract_Method_AddDepositorSlashing() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.AddDepositorSlashing
}

func GetContract_Method_SetNilBlock() string {
	return "setNilBlock"
}
func GetContract_Method_ResetNilBlock() string {
	return "resetNilBlock"
}

func GetContract_Method_AddDepositorReward() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Validators.AddDepositorReward
}

func GetContract_Method_GetDepositorRewards() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetDepositorRewards
}

func GetContract_Method_GetDepositorSlashings() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetDepositorSlashings
}

func GetContract_Method_GetWithdrawalBlock() string {
	return SystemContractsData[stakingContract].Contracts.Methods.Deposits.GetWithdrawalBlock
}

func (sf Contract) Address() common.Address {
	return sf.CallerAddress
}

func GetContract_Method_ChangeValidator() string {
	return "changeValidator"
}

func GetContract_Method_InitiateChangeDepositor() string {
	return "initiateChangeDepositor"
}

func GetContract_Method_CompleteChangeDepositor() string {
	return "completeChangeDepositor"
}

func GetContract_Method_IncreaseDeposit() string {
	return "increaseDeposit"
}

func GetContract_Method_InitiatePartialWithdrawal() string {
	return "initiatePartialWithdrawal"
}

func GetContract_Method_CompletePartialWithdrawal() string {
	return "completePartialWithdrawal"
}

func GetContract_Method_GetStakingDetails() string {
	return "getStakingDetails"
}
