package config

import (
	"log"
	"math/big"
	"os"
	"time"

	"github.com/elastos/Elastos.ELA/common"
)

var (
	Parameters ConfigParams
	Version    string

	MainNet = ChainParams{
		Name:               "MainNet",
		PowLimit:           powLimit,
		PowLimitBits:       0x1f0008ff,
		TargetTimePerBlock: time.Minute * 2,
		TargetTimespan:     time.Minute * 2 * 720,
		AdjustmentFactor:   int64(4),
		MaxOrphanBlocks:    10000,
		MinMemoryNodes:     20160,
		CoinbaseLockTime:   100,
	}

	TestNet = ChainParams{
		Name:               "TestNet",
		PowLimit:           powLimit,
		PowLimitBits:       0x1e1da5ff,
		TargetTimePerBlock: time.Second * 10,
		TargetTimespan:     time.Second * 10 * 10,
		AdjustmentFactor:   int64(4),
		MaxOrphanBlocks:    10000,
		MinMemoryNodes:     20160,
		CoinbaseLockTime:   100,
	}

	RegNet = ChainParams{
		Name:               "RegNet",
		PowLimit:           powLimit,
		PowLimitBits:       0x207fffff,
		TargetTimePerBlock: time.Second * 1,
		TargetTimespan:     time.Second * 1 * 10,
		AdjustmentFactor:   int64(4),
		MaxOrphanBlocks:    10000,
		MinMemoryNodes:     20160,
		CoinbaseLockTime:   100,
	}
)

type PowConfiguration struct {
	PayToAddr  string `json:"PayToAddr"`
	AutoMining bool   `json:"AutoMining"`
	MinerInfo  string `json:"MinerInfo"`
	MinTxFee   int    `json:"MinTxFee"`
	ActiveNet  string `json:"ActiveNet"`
}

type Configuration struct {
	Magic                uint32               `json:"Magic"`
	FoundationAddress    string               `json:"FoundationAddress"`
	Version              int                  `json:"Version"`
	SeedList             []string             `json:"SeedList"`
	HttpRestPort         int                  `json:"HttpRestPort"`
	MinCrossChainTxFee   int                  `json:"MinCrossChainTxFee"`
	RestCertPath         string               `json:"RestCertPath"`
	RestKeyPath          string               `json:"RestKeyPath"`
	HttpInfoPort         uint16               `json:"HttpInfoPort"`
	HttpInfoStart        bool                 `json:"HttpInfoStart"`
	OpenService          bool                 `json:"OpenService"`
	HttpWsPort           int                  `json:"HttpWsPort"`
	WsHeartbeatInterval  time.Duration        `json:"WsHeartbeatInterval"`
	HttpJsonPort         int                  `json:"HttpJsonPort"`
	OauthServerUrl       string               `json:"OauthServerUrl"`
	NoticeServerUrl      string               `json:"NoticeServerUrl"`
	NodePort             uint16               `json:"NodePort"`
	NodeOpenPort         uint16               `json:"NodeOpenPort"`
	PrintLevel           uint8                `json:"PrintLevel"`
	IsTLS                bool                 `json:"IsTLS"`
	CertPath             string               `json:"CertPath"`
	KeyPath              string               `json:"KeyPath"`
	CAPath               string               `json:"CAPath"`
	MultiCoreNum         uint                 `json:"MultiCoreNum"`
	MaxLogsSize          int64                `json:"MaxLogsSize"`
	MaxPerLogSize        int64                `json:"MaxPerLogSize"`
	MaxTxsInBlock        int                  `json:"MaxTransactionInBlock"`
	MaxBlockSize         int                  `json:"MaxBlockSize"`
	PowConfiguration     PowConfiguration     `json:"PowConfiguration"`
	EnableArbiter        bool                 `json:"EnableArbiter"`
	ArbiterConfiguration ArbiterConfiguration `json:"ArbiterConfiguration"`
}

type ArbiterConfiguration struct {
	Name             string `json:"Name"`
	Magic            uint32 `json:"Magic"`
	NodePort         uint16 `json:"NodePort"`
	ProtocolVersion  uint32 `json:"ProtocolVersion"`
	Services         uint64 `json:"Services"`
	PrintLevel       uint8  `json:"PrintLevel"`
	SignTolerance    uint64 `json:"SignTolerance"`
	MaxLogsSize      int64  `json:"MaxLogsSize"`
	MaxPerLogSize    int64  `json:"MaxPerLogSize"`
	MaxConnections   int    `json:"MaxConnections"`
	MajorityCount    uint32 `json:"MajorityCount"`
	ArbitratorsCount uint32 `json:"ArbitratorsCount"`
	CandidatesCount  uint32 `json:"CandidatesCount"`
}

type Seed struct {
	PublicKey string `json:"PublicKey"`
	Address   string `json:"Address"`
}

type ConfigFile struct {
	Configuration `json:"Configuration"`
}

type ChainParams struct {
	Name               string        `json:"Name"`
	PowLimit           *big.Int      `json:"PowLimit"`
	PowLimitBits       uint32        `json:"PowLimitBits"`
	TargetTimePerBlock time.Duration `json:"TargetTimePerBlock"`
	TargetTimespan     time.Duration `json:"TargetTimespan"`
	AdjustmentFactor   int64         `json:"AdjustmentFactor"`
	MaxOrphanBlocks    int           `json:"MaxOrphanBlocks"`
	MinMemoryNodes     uint32        `json:"MinMemoryNodes"`
	CoinbaseLockTime   uint32        `json:"CoinbaseLockTime"`
}

type ConfigParams struct {
	*Configuration
	ChainParam *ChainParams
}

func (config *Configuration) GetArbiterID() []byte {
	publicKey, err := common.HexStringToBytes(config.ArbiterConfiguration.Name)
	if err != nil || len(publicKey) != 33 {
		log.Fatalf("get arbiter public key error %v", err)
		os.Exit(1)
	}

	return publicKey
}
