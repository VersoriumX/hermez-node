package apitypes

import (
	"encoding/json"
	"time"

	"github.com/hermeznetwork/hermez-node/common"
	"github.com/iden3/go-iden3-crypto/babyjub"
)

// PoolL2TxAPI represents a L2 Tx pool with extra metadata used by the API
type PoolL2TxAPI struct {
	ItemID               uint64
	TxID                 common.TxID
	Type                 common.TxType
	FromIdx              HezIdx
	EffectiveFromEthAddr *HezEthAddr
	EffectiveFromBJJ     *HezBJJ
	ToIdx                *HezIdx
	EffectiveToEthAddr   *HezEthAddr
	EffectiveToBJJ       *HezBJJ
	Amount               BigIntStr
	Fee                  common.FeeSelector
	Nonce                common.Nonce
	State                common.PoolL2TxState
	MaxNumBatch          uint32
	Info                 *string
	ErrorCode            *int
	ErrorType            *string
	Signature            babyjub.SignatureComp
	Timestamp            time.Time
	RqFromIdx            *HezIdx
	RqToIdx              *HezIdx
	RqToEthAddr          *HezEthAddr
	RqToBJJ              *HezBJJ
	RqTokenID            *common.TokenID
	RqAmount             *BigIntStr
	RqFee                *common.FeeSelector
	RqNonce              *common.Nonce
	Token                struct {
		TokenID          common.TokenID
		TokenItemID      uint64
		TokenEthBlockNum int64
		TokenEthAddr     HezEthAddr
		TokenName        string
		TokenSymbol      string
		TokenDecimals    uint64
		TokenUSD         *float64
		TokenUSDUpdate   *time.Time
	}
}

// MarshalJSON is used to convert PoolL2TxAPI in JSON
func (tx PoolL2TxAPI) MarshalJSON() ([]byte, error) {
	type jsonToken struct {
		TokenID          common.TokenID `json:"id"`
		TokenItemID      uint64         `json:"itemId"`
		TokenEthBlockNum int64          `json:"ethereumBlockNum"`
		TokenEthAddr     HezEthAddr     `json:"ethereumAddress"`
		TokenName        string         `json:"name"`
		TokenSymbol      string         `json:"symbol"`
		TokenDecimals    uint64         `json:"decimals"`
		TokenUSD         *float64       `json:"USD"`
		TokenUSDUpdate   *time.Time     `json:"fiatUpdate"`
	}
	type jsonFormat struct {
		ItemID               uint64                `json:"itemId"`
		TxID                 common.TxID           `json:"id"`
		Type                 common.TxType         `json:"type"`
		FromIdx              HezIdx                `json:"fromAccountIndex"`
		EffectiveFromEthAddr *HezEthAddr           `json:"fromHezEthereumAddress"`
		EffectiveFromBJJ     *HezBJJ               `json:"fromBJJ"`
		ToIdx                *HezIdx               `json:"toAccountIndex"`
		EffectiveToEthAddr   *HezEthAddr           `json:"toHezEthereumAddress"`
		EffectiveToBJJ       *HezBJJ               `json:"toBJJ"`
		Amount               BigIntStr             `json:"amount"`
		Fee                  common.FeeSelector    `json:"fee"`
		Nonce                common.Nonce          `json:"nonce"`
		State                common.PoolL2TxState  `json:"state"`
		MaxNumBatch          uint32                `json:"maxNumBatch"`
		Info                 *string               `json:"info"`
		ErrorCode            *int                  `json:"errorCode"`
		ErrorType            *string               `json:"errorType"`
		Signature            babyjub.SignatureComp `json:"signature"`
		Timestamp            time.Time             `json:"timestamp"`
		RqFromIdx            *HezIdx               `json:"requestFromAccountIndex"`
		RqToIdx              *HezIdx               `json:"requestToAccountIndex"`
		RqToEthAddr          *HezEthAddr           `json:"requestToHezEthereumAddress"`
		RqToBJJ              *HezBJJ               `json:"requestToBJJ"`
		RqTokenID            *common.TokenID       `json:"requestTokenId"`
		RqAmount             *BigIntStr            `json:"requestAmount"`
		RqFee                *common.FeeSelector   `json:"requestFee"`
		RqNonce              *common.Nonce         `json:"requestNonce"`
		Token                jsonToken             `json:"token"`
	}
	toMarshal := jsonFormat{
		ItemID:               tx.ItemID,
		TxID:                 tx.TxID,
		Type:                 tx.Type,
		FromIdx:              tx.FromIdx,
		EffectiveFromEthAddr: tx.EffectiveFromEthAddr,
		EffectiveFromBJJ:     tx.EffectiveFromBJJ,
		ToIdx:                tx.ToIdx,
		EffectiveToEthAddr:   tx.EffectiveToEthAddr,
		EffectiveToBJJ:       tx.EffectiveToBJJ,
		Amount:               tx.Amount,
		Fee:                  tx.Fee,
		Nonce:                tx.Nonce,
		State:                tx.State,
		MaxNumBatch:          tx.MaxNumBatch,
		Info:                 tx.Info,
		ErrorCode:            tx.ErrorCode,
		ErrorType:            tx.ErrorType,
		Signature:            tx.Signature,
		Timestamp:            tx.Timestamp,
		RqFromIdx:            tx.RqFromIdx,
		RqToIdx:              tx.RqToIdx,
		RqToEthAddr:          tx.RqToEthAddr,
		RqToBJJ:              tx.RqToBJJ,
		RqTokenID:            tx.RqTokenID,
		RqAmount:             tx.RqAmount,
		RqFee:                tx.RqFee,
		RqNonce:              tx.RqNonce,
		Token: jsonToken{
			TokenID:          tx.Token.TokenID,
			TokenItemID:      tx.Token.TokenItemID,
			TokenEthBlockNum: tx.Token.TokenEthBlockNum,
			TokenEthAddr:     tx.Token.TokenEthAddr,
			TokenName:        tx.Token.TokenName,
			TokenSymbol:      tx.Token.TokenSymbol,
			TokenDecimals:    tx.Token.TokenDecimals,
			TokenUSD:         tx.Token.TokenUSD,
			TokenUSDUpdate:   tx.Token.TokenUSDUpdate,
		},
	}
	return json.Marshal(toMarshal)
}

// UnmarshalJSON is used to create a PoolL2TxAPI from JSON data
func (tx *PoolL2TxAPI) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, tx)
	if err != nil {
		return err
	}
	return nil
}
