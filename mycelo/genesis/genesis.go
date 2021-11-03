package genesis

import (
	"bytes"
	"fmt"
	"github.com/celo-org/celo-blockchain/accounts/keystore"
	"github.com/celo-org/celo-blockchain/common"
	"github.com/celo-org/celo-blockchain/common/decimal/token"
	"github.com/celo-org/celo-blockchain/core"
	"github.com/celo-org/celo-blockchain/core/types"
	"github.com/celo-org/celo-blockchain/crypto"
	blscrypto "github.com/celo-org/celo-blockchain/crypto/bls"
	"github.com/celo-org/celo-blockchain/log"
	"github.com/celo-org/celo-blockchain/mycelo/env"
	"github.com/celo-org/celo-blockchain/params"
	"gopkg.in/urfave/cli.v1"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"time"

	"github.com/celo-org/celo-blockchain/rlp"
)

// Keccak256 of "The Times 09/Apr/2020 With $2.3 Trillion Injection, Fed’s Plan Far Exceeds Its 2008 Rescue"
var genesisMsgHash = bytes.Repeat([]byte{0x00}, 32)

//var genesisMsgHash = common.HexToHash("ecc833a7747eaa8327335e8e0c6b6d8aa3a38d0063591e43ce116ccf5c89753e")
var Validators_my []env.Account
var Groups_my []env.Account
var Admin_my env.Account

// CreateCommonGenesisConfig generates a config starting point which templates can then customize further
func CreateCommonGenesisConfig(chainID *big.Int, adminAccountAddress common.Address, istanbulConfig params.IstanbulConfig) *Config {
	genesisConfig := BaseConfig()
	genesisConfig.ChainID = chainID
	genesisConfig.GenesisTimestamp = uint64(time.Now().Unix())
	genesisConfig.Istanbul = istanbulConfig
	genesisConfig.Hardforks = HardforkConfig{
		ChurritoBlock: common.Big0,
		DonutBlock:    common.Big0,
	}

	// Make Admin_my account manager of Governance & Reserve
	adminMultisig := MultiSigParameters{
		Signatories:                      []common.Address{adminAccountAddress},
		NumRequiredConfirmations:         1,
		NumInternalRequiredConfirmations: 1,
	}

	genesisConfig.ReserveSpenderMultiSig = adminMultisig
	genesisConfig.GovernanceApproverMultiSig = adminMultisig

	// Ensure nothing is frozen
	genesisConfig.GoldToken.Frozen = false
	genesisConfig.StableToken.Frozen = false
	genesisConfig.Exchange.Frozen = false
	genesisConfig.Reserve.FrozenAssetsDays = 0
	genesisConfig.EpochRewards.Frozen = false

	return genesisConfig
}

func FundAccounts(genesisConfig *Config, accounts []env.Account) {
	cusdBalances := make([]Balance, len(accounts))
	ceurBalances := make([]Balance, len(accounts))
	goldBalances := make([]Balance, len(accounts))
	for i, acc := range accounts {
		cusdBalances[i] = Balance{Account: acc.Address, Amount: (*big.Int)(token.MustNew("50000"))} // 50k cUSD
		ceurBalances[i] = Balance{Account: acc.Address, Amount: (*big.Int)(token.MustNew("50000"))} // 50k cEUR
		goldBalances[i] = Balance{Account: acc.Address, Amount: (*big.Int)(token.MustNew("50000"))} // 50k CELO
	}
	genesisConfig.StableTokenEUR.InitialBalances = ceurBalances
	genesisConfig.StableToken.InitialBalances = cusdBalances
	genesisConfig.GoldToken.InitialBalances = goldBalances
}

// GenerateGenesis will create a new genesis block with full celo blockchain already configured
func GenerateGenesis(ctx *cli.Context, accounts *env.AccountsConfig, cfg *Config, contractsBuildPath string) (*core.Genesis, error) {
	Validators_my = loadValidators(ctx)
	Groups_my = loadGroups(ctx)
	Admin_my = Validators_my[0]
	extraData, err := generateGenesisExtraData(Validators_my)
	//fmt.Println("extraData: ", hexutil.Encode(extraData))
	if err != nil {
		return nil, err
	}

	genesisAlloc, err := generateGenesisState(accounts, cfg, contractsBuildPath)
	if err != nil {
		return nil, err
	}

	return &core.Genesis{
		Config:    cfg.ChainConfig(),
		ExtraData: extraData,
		Coinbase:  Admin_my.Address, //todo  zhangwei
		Timestamp: cfg.GenesisTimestamp,
		Alloc:     genesisAlloc,
	}, nil
}

func generateGenesisExtraData(validatorAccounts []env.Account) ([]byte, error) {
	addresses := make([]common.Address, len(validatorAccounts))
	blsKeys := make([]blscrypto.SerializedPublicKey, len(validatorAccounts))

	for i := 0; i < len(validatorAccounts); i++ {
		var err error
		addresses[i] = validatorAccounts[i].Address
		blsKeys[i], err = validatorAccounts[i].BLSPublicKey()
		if err != nil {
			return nil, err
		}
	}

	istExtra := types.IstanbulExtra{
		AddedValidators:           addresses,
		AddedValidatorsPublicKeys: blsKeys,
		RemovedValidators:         big.NewInt(0),
		Seal:                      []byte{},
		AggregatedSeal:            types.IstanbulAggregatedSeal{},
		ParentAggregatedSeal:      types.IstanbulAggregatedSeal{},
	}

	payload, err := rlp.EncodeToBytes(&istExtra)
	if err != nil {
		return nil, err
	}

	var extraBytes []byte
	extraBytes = append(extraBytes, genesisMsgHash...)
	extraBytes = append(extraBytes, payload...)

	return extraBytes, nil
}
func loadValidators(ctx *cli.Context) []env.Account {
	keyDir := ""
	if ctx.IsSet("ValidatorsKeyDir") {
		keyDir = ctx.String("ValidatorsKeyDir")
	}
	return loadPrivate(keyDir, 4)
}
func loadGroups(ctx *cli.Context) []env.Account {
	keyDir := ""
	if ctx.IsSet("GroupsKeyDir") {
		keyDir = ctx.String("GroupsKeyDir")
	}
	return loadPrivate(keyDir, 1)
}
func loadPrivate(keyDir string, num int) []env.Account {
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return nil
	}
	accounts := make([]env.Account, num)
	for i, fi := range files {
		path := filepath.Join(keyDir, fi.Name())
		keyjson, err := ioutil.ReadFile(path)
		if err != nil {
			log.Error("loadPrivate ReadFile", fmt.Errorf("failed to read the keyfile at '%s': %v", path, err))
		}
		key, err := keystore.DecryptKey(keyjson, "")
		if err != nil {
			log.Error("loadPrivate DecryptKey", fmt.Errorf("error decrypting key: %v", err))
		}
		priKey1 := key.PrivateKey
		publicAddr := crypto.PubkeyToAddress(priKey1.PublicKey)
		var addr common.Address
		addr.SetBytes(publicAddr.Bytes())
		accounts[i] = env.Account{
			Address:    addr,
			PrivateKey: priKey1,
		}
	}
	return accounts
}
