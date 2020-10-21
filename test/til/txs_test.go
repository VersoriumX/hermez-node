package til

import (
	"math/big"
	"testing"

	"github.com/hermeznetwork/hermez-node/common"
	"github.com/hermeznetwork/hermez-node/eth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBlocks(t *testing.T) {
	set := `
		Type: Blockchain
		RegisterToken(1)
		RegisterToken(2)
		RegisterToken(3)
	
		CreateAccountDeposit(1) A: 10
		CreateAccountDeposit(2) A: 20
		CreateAccountDeposit(1) B: 5
		CreateAccountDeposit(1) C: 5
		CreateAccountDepositTransfer(1) D-A: 15, 10 (3)

		> batchL1
		> batchL1

		Transfer(1) A-B: 6 (1)
		Transfer(1) B-D: 3 (1)
		Transfer(1) A-D: 1 (1)

		// set new batch
		> batch
		CreateAccountDepositCoordinator(1) E
		CreateAccountDepositCoordinator(2) B

		DepositTransfer(1) A-B: 15, 10 (1)
		Transfer(1) C-A : 3 (1)
		Transfer(2) A-B: 15 (1)
		Transfer(1) A-E: 1 (1)

		CreateAccountDeposit(1) User0: 20
		CreateAccountDeposit(3) User1: 20
		CreateAccountDepositCoordinator(1) User1
		CreateAccountDepositCoordinator(3) User0
		> batchL1
		Transfer(1) User0-User1: 15 (1)
		Transfer(3) User1-User0: 15 (1)
		Transfer(1) A-C: 1 (1)

		> batchL1

		Transfer(1) User1-User0: 1 (1)

		> block

		// Exits
		Transfer(1) A-B: 1 (1)
		Exit(1) A: 5
		
		> batch
		> block

		// this transaction should not be generated, as it's after last
		// batch and last block
		Transfer(1) User1-User0: 1 (1)
	`
	tc := NewContext(eth.RollupConstMaxL1UserTx)
	blocks, err := tc.GenerateBlocks(set)
	require.Nil(t, err)
	assert.Equal(t, 2, len(blocks))
	assert.Equal(t, 5, len(blocks[0].Batches))
	assert.Equal(t, 1, len(blocks[1].Batches))
	assert.Equal(t, 8, len(blocks[0].L1UserTxs))
	assert.Equal(t, 4, len(blocks[0].Batches[3].L1CoordinatorTxs))
	assert.Equal(t, 0, len(blocks[1].L1UserTxs))

	// Check expected values generated by each line
	// #0: Deposit(1) A: 10
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[0], common.TxTypeCreateAccountDeposit, 1, "A", "", big.NewInt(10), nil)
	// #1: Deposit(2) A: 20
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[1], common.TxTypeCreateAccountDeposit, 2, "A", "", big.NewInt(20), nil)
	// // #2: Deposit(1) A: 20
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[2], common.TxTypeCreateAccountDeposit, 1, "B", "", big.NewInt(5), nil)
	// // #3: CreateAccountDeposit(1) C: 5
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[3], common.TxTypeCreateAccountDeposit, 1, "C", "", big.NewInt(5), nil)
	// // #4: CreateAccountDepositTransfer(1) D-A: 15, 10 (3)
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[4], common.TxTypeCreateAccountDepositTransfer, 1, "D", "A", big.NewInt(15), big.NewInt(10))
	// #5: Transfer(1) A-B: 6 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[2].L2Txs[0], common.TxTypeTransfer, 1, "A", "B", big.NewInt(6), common.BatchNum(2), common.Nonce(1))
	// #6: Transfer(1) B-D: 3 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[2].L2Txs[1], common.TxTypeTransfer, 1, "B", "D", big.NewInt(3), common.BatchNum(2), common.Nonce(1))
	// #7: Transfer(1) A-D: 1 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[2].L2Txs[2], common.TxTypeTransfer, 1, "A", "D", big.NewInt(1), common.BatchNum(2), common.Nonce(2))
	// change of Batch
	// #8: DepositTransfer(1) A-B: 15, 10 (1)
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[5], common.TxTypeDepositTransfer, 1, "A", "B", big.NewInt(15), big.NewInt(10))
	// #10: Transfer(1) C-A : 3 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[3].L2Txs[0], common.TxTypeTransfer, 1, "C", "A", big.NewInt(3), common.BatchNum(3), common.Nonce(1))
	// #11: Transfer(2) A-B: 15 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[3].L2Txs[1], common.TxTypeTransfer, 2, "A", "B", big.NewInt(15), common.BatchNum(3), common.Nonce(1))
	// #12: Deposit(1) User0: 20
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[6], common.TxTypeCreateAccountDeposit, 1, "User0", "", big.NewInt(20), nil)
	// // #13: Deposit(3) User1: 20
	tc.checkL1TxParams(t, blocks[0].L1UserTxs[7], common.TxTypeCreateAccountDeposit, 3, "User1", "", big.NewInt(20), nil)
	// #14: Transfer(1) User0-User1: 15 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[4].L2Txs[0], common.TxTypeTransfer, 1, "User0", "User1", big.NewInt(15), common.BatchNum(4), common.Nonce(1))
	// #15: Transfer(3) User1-User0: 15 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[4].L2Txs[1], common.TxTypeTransfer, 3, "User1", "User0", big.NewInt(15), common.BatchNum(4), common.Nonce(1))
	// #16: Transfer(1) A-C: 1 (1)
	tc.checkL2TxParams(t, blocks[0].Batches[4].L2Txs[2], common.TxTypeTransfer, 1, "A", "C", big.NewInt(1), common.BatchNum(4), common.Nonce(4))
	// change of Batch
	// #17: Transfer(1) User1-User0: 1 (1)
	tc.checkL2TxParams(t, blocks[1].Batches[0].L2Txs[0], common.TxTypeTransfer, 1, "User1", "User0", big.NewInt(1), common.BatchNum(5), common.Nonce(1))
	// change of Block (implies also a change of batch)
	// #18: Transfer(1) A-B: 1 (1)
	tc.checkL2TxParams(t, blocks[1].Batches[0].L2Txs[1], common.TxTypeTransfer, 1, "A", "B", big.NewInt(1), common.BatchNum(5), common.Nonce(5))
}

func (tc *Context) checkL1TxParams(t *testing.T, tx common.L1Tx, typ common.TxType, tokenID common.TokenID, from, to string, loadAmount, amount *big.Int) {
	assert.Equal(t, typ, tx.Type)
	if tx.FromIdx != common.Idx(0) {
		assert.Equal(t, tc.Users[from].Accounts[tokenID].Idx, tx.FromIdx)
	}
	assert.Equal(t, tc.Users[from].Addr.Hex(), tx.FromEthAddr.Hex())
	assert.Equal(t, tc.Users[from].BJJ.Public(), tx.FromBJJ)
	if tx.ToIdx != common.Idx(0) {
		assert.Equal(t, tc.Users[to].Accounts[tokenID].Idx, tx.ToIdx)
	}
	if loadAmount != nil {
		assert.Equal(t, loadAmount, tx.LoadAmount)
	}
	if amount != nil {
		assert.Equal(t, amount, tx.Amount)
	}
}
func (tc *Context) checkL2TxParams(t *testing.T, tx common.L2Tx, typ common.TxType, tokenID common.TokenID, from, to string, amount *big.Int, batchNum common.BatchNum, nonce common.Nonce) {
	assert.Equal(t, typ, tx.Type)
	assert.Equal(t, tc.Users[from].Accounts[tokenID].Idx, tx.FromIdx)
	if tx.Type != common.TxTypeExit {
		assert.Equal(t, tc.Users[to].Accounts[tokenID].Idx, tx.ToIdx)
	}
	if amount != nil {
		assert.Equal(t, amount, tx.Amount)
	}
	assert.Equal(t, batchNum, tx.BatchNum)
	assert.Equal(t, nonce, tx.Nonce)
}

func TestGeneratePoolL2Txs(t *testing.T) {
	set := `
		Type: Blockchain
		RegisterToken(1)
		RegisterToken(2)
		RegisterToken(3)
	
		CreateAccountDeposit(1) A: 10
		CreateAccountDeposit(2) A: 20
		CreateAccountDeposit(1) B: 5
		CreateAccountDeposit(1) C: 5
		CreateAccountDeposit(1) User0: 5
		CreateAccountDeposit(1) User1: 0
		CreateAccountDeposit(3) User0: 0
		CreateAccountDeposit(3) User1: 5
		CreateAccountDeposit(2) B: 5
		CreateAccountDeposit(2) D: 0
		> batchL1
		> batchL1
	`
	tc := NewContext(eth.RollupConstMaxL1UserTx)
	_, err := tc.GenerateBlocks(set)
	require.Nil(t, err)
	set = `
		Type: PoolL2
		PoolTransfer(1) A-B: 6 (1)
		PoolTransfer(1) B-C: 3 (1)
		PoolTransfer(1) C-A: 3 (1)
		PoolTransfer(1) A-B: 1 (1)
		PoolTransfer(2) A-B: 15 (1)
		PoolTransfer(1) User0-User1: 15 (1)
		PoolTransfer(3) User1-User0: 15 (1)
		PoolTransfer(2) B-D: 3 (1)
		PoolExit(1) A: 3
	`
	poolL2Txs, err := tc.GeneratePoolL2Txs(set)
	require.Nil(t, err)
	assert.Equal(t, 9, len(poolL2Txs))
	assert.Equal(t, common.TxTypeTransfer, poolL2Txs[0].Type)
	assert.Equal(t, common.TxTypeExit, poolL2Txs[8].Type)
	assert.Equal(t, tc.Users["B"].Addr.Hex(), poolL2Txs[0].ToEthAddr.Hex())
	assert.Equal(t, tc.Users["B"].BJJ.Public().String(), poolL2Txs[0].ToBJJ.String())
	assert.Equal(t, tc.Users["User1"].Addr.Hex(), poolL2Txs[5].ToEthAddr.Hex())
	assert.Equal(t, tc.Users["User1"].BJJ.Public().String(), poolL2Txs[5].ToBJJ.String())

	assert.Equal(t, common.Nonce(1), poolL2Txs[0].Nonce)
	assert.Equal(t, common.Nonce(2), poolL2Txs[3].Nonce)
	assert.Equal(t, common.Nonce(3), poolL2Txs[8].Nonce)

	// load another set in the same Context
	set = `
		Type: PoolL2
		PoolTransfer(1) A-B: 6 (1)
		PoolTransfer(1) B-C: 3 (1)
		PoolTransfer(1) A-C: 3 (1)
	`
	poolL2Txs, err = tc.GeneratePoolL2Txs(set)
	require.Nil(t, err)
	assert.Equal(t, common.Nonce(4), poolL2Txs[0].Nonce)
	assert.Equal(t, common.Nonce(2), poolL2Txs[1].Nonce)
	assert.Equal(t, common.Nonce(5), poolL2Txs[2].Nonce)
}

func TestGenerateErrors(t *testing.T) {
	// unregistered token
	set := `Type: Blockchain
		CreateAccountDeposit(1) A: 5
		> batchL1
		`
	tc := NewContext(eth.RollupConstMaxL1UserTx)
	_, err := tc.GenerateBlocks(set)
	assert.Equal(t, "Line 2: Can not process CreateAccountDeposit: TokenID 1 not registered, last registered TokenID: 0", err.Error())

	// ensure RegisterToken sequentiality and not using 0
	set = `
		Type: Blockchain
		RegisterToken(0)
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Equal(t, "Line 2: RegisterToken can not register TokenID 0", err.Error())

	set = `
		Type: Blockchain
		RegisterToken(2)
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Equal(t, "Line 2: RegisterToken TokenID should be sequential, expected TokenID: 1, defined TokenID: 2", err.Error())

	set = `
		Type: Blockchain
		RegisterToken(1)
		RegisterToken(2)
		RegisterToken(3)
		RegisterToken(5)
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Equal(t, "Line 5: RegisterToken TokenID should be sequential, expected TokenID: 4, defined TokenID: 5", err.Error())

	// check transactions when account is not created yet
	set = `
		Type: Blockchain
		RegisterToken(1)
		CreateAccountDeposit(1) A: 10
		> batchL1
		CreateAccountDeposit(1) B
		Transfer(1) A-B: 6 (1)
		> batch
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Equal(t, "Line 5: CreateAccountDeposit(1)BTransfer(1) A-B: 6 (1)\n, err: Expected ':', found 'Transfer'", err.Error())
	set = `
		Type: Blockchain
		RegisterToken(1)
		CreateAccountDeposit(1) A: 10
		> batchL1
		CreateAccountDepositCoordinator(1) B
		> batchL1
		> batch
		Transfer(1) A-B: 6 (1)
		> batch
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Nil(t, err)

	// check nonces
	set = `
		Type: Blockchain
		RegisterToken(1)
		CreateAccountDeposit(1) A: 10
		> batchL1
		CreateAccountDepositCoordinator(1) B
		> batchL1
		Transfer(1) A-B: 6 (1)
		Transfer(1) A-B: 6 (1) // on purpose this is moving more money that what it has in the account, Til should not fail
		Transfer(1) B-A: 6 (1)
		Exit(1) A: 3
		> batch
	`
	tc = NewContext(eth.RollupConstMaxL1UserTx)
	_, err = tc.GenerateBlocks(set)
	require.Nil(t, err)
	assert.Equal(t, common.Nonce(3), tc.Users["A"].Accounts[common.TokenID(1)].Nonce)
	assert.Equal(t, common.Idx(256), tc.Users["A"].Accounts[common.TokenID(1)].Idx)
	assert.Equal(t, common.Nonce(1), tc.Users["B"].Accounts[common.TokenID(1)].Nonce)
	assert.Equal(t, common.Idx(257), tc.Users["B"].Accounts[common.TokenID(1)].Idx)
}
