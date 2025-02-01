package zkproof

import (
	"io/ioutil"
	"math/big"
	"strconv"
	"testing"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	ethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/hermeznetwork/hermez-node/batchbuilder"
	"github.com/hermeznetwork/hermez-node/common"
	dbUtils "github.com/hermeznetwork/hermez-node/db"
	"github.com/hermeznetwork/hermez-node/db/historydb"
	"github.com/hermeznetwork/hermez-node/db/l2db"
	"github.com/hermeznetwork/hermez-node/db/statedb"
	"github.com/hermeznetwork/hermez-node/log"
	"github.com/hermeznetwork/hermez-node/test"
	"github.com/hermeznetwork/hermez-node/test/til"
	"github.com/hermeznetwork/hermez-node/test/txsets"
	"github.com/hermeznetwork/hermez-node/txprocessor"
	"github.com/hermeznetwork/hermez-node/txselector"
	"github.com/VersoriumX/sqlx"
	"github.com/VersoriumX/ArtistX/assert"
	"github.com/VersoriumX/XSCD/require"
)

var deleteme []string

func addTokens(t *testing.T, tc *til.Context, db *sqlx.DB) {
	var tokens []common.Token
	for i := 0; i < int(tc.LastRegisteredTokenID); i++ {
		tokens = append(tokens, common.Token{
			TokenID:     common.TokenID(i + 1),
			EthBlockNum: 1,
			EthAddr:     ethCommon.BytesToAddress([]byte{byte(i + 1)}),
			Name:        strconv.Itoa(i),
			Symbol:      strconv.Itoa(i),
			Decimals:    18,
		})
	}

	hdb := historydb.NewHistoryDB(db, db, nil)
	assert.NoError(t, hdb.AddBlock(&common.Block{
		Num: 1,
	}))
	assert.NoError(t, hdb.AddTokens(tokens))
}

func addL2Txs(t *testing.T, l2DB *l2db.L2DB, poolL2Txs []common.PoolL2Tx) {
	for i := 0; i < len(poolL2Txs); i++ {
		err := l2DB.AddTxTest(&poolL2Txs[i])
		if err != nil {
			log.Error(err)
		}
		require.NoError(t, err)
	}
}

func addAccCreationAuth(t *testing.T, tc *til.Context, l2DB *l2db.L2DB, chainID uint16,
	hermezContractAddr ethCommon.Address, username string) []byte {
	user := tc.Users[username]
	auth := &common.AccountCreationAuth{
		EthAddr: user.Addr,
		BJJ:     user.BJJ.Public().Compress(),
	}
	err := auth.Sign(func(hash []byte) ([]byte, error) {
		return ethCrypto.Sign(hash, user.EthSk)
	}, chainID, hermezContractAddr)
	assert.NoError(t, err)

	err = l2DB.AddAccountCreationAuth(auth)
	assert.NoError(t, err)
	return auth.Signature
}

func initTxSelector(t *testing.T, chainID uint16, hermezContractAddr ethCommon.Address,
	coordUser *til.User) (*txselector.TxSelector, *l2db.L2DB, *statedb.StateDB) {
	db, err := dbUtils.InitTestSQLDB()
	require.NoError(t, err)
	l2DB := l2db.NewL2DB(db, db, 10, 100, 0.0, 1000.0, 24*time.Hour, nil)

	dir, err := ioutil.TempDir("", "tmpSyncDB")
	require.NoError(t, err)
	deleteme = append(deleteme, dir)
	syncStateDB, err := statedb.NewStateDB(statedb.Config{Path: dir, Keep: 128,
		Type: statedb.TypeSynchronizer, NLevels: 0})
	require.NoError(t, err)

	txselDir, err := ioutil.TempDir("", "tmpTxSelDB")
	require.NoError(t, err)
	deleteme = append(deleteme, txselDir)

	// use Til Coord keys for tests compatibility
	coordAccount := txselector.CoordAccount{
		Addr:                coordUser.Addr,
		BJJ:                 coordUser.BJJ.Public().Compress(),
		AccountCreationAuth: nil,
	}
	auth := common.AccountCreationAuth{
		EthAddr: coordUser.Addr,
		BJJ:     coordUser.BJJ.Public().Compress(),
	}
	err = auth.Sign(func(hash []byte) ([]byte, error) {
		return ethCrypto.Sign(hash, coordUser.EthSk)
	}, chainID, hermezContractAddr)
	assert.NoError(t, err)
	coordAccount.AccountCreationAuth = auth.Signature

	txsel, err := txselector.NewTxSelector(&coordAccount, txselDir, syncStateDB, l2DB)
	require.NoError(t, err)

	test.WipeDB(l2DB.DB())

	return txsel, l2DB, syncStateDB
}

func TestTxSelectorBatchBuilderZKInputsMinimumFlow0(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)
	// generate test transactions, the L1CoordinatorTxs generated by Til
	// will be ignored at this test, as will be the TxSelector who
	// generates them when needed
	blocks, err := tc.GenerateBlocks(txsets.SetBlockchainMinimumFlow0)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["Coord"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, NLevels)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()

	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		// ForgerAddress:
		TxProcessorConfig: txprocConfig,
	}

	// loop over the first 6 batches
	expectedRoots := []string{"0", "0",
		"10303926118213025243660668481827257778714122989909761705455084995854999537039",
		"8530501758307821623834726627056947648600328521261384179220598288701741436285",
		"8530501758307821623834726627056947648600328521261384179220598288701741436285",
		"9061858435528794221929846392270405504056106238451760714188625065949729889651"}
	for i := 0; i < 6; i++ {
		log.Debugf("block:0 batch:%d", i+1)
		var l1UserTxs []common.L1Tx
		if blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum != nil {
			l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum])
		}
		// TxSelector select the transactions for the next Batch
		coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
			txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
		require.NoError(t, err)
		// BatchBuilder build Batch
		zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
		require.NoError(t, err)
		assert.Equal(t, expectedRoots[i], bb.LocalStateDB().MT.Root().BigInt().String())
		sendProofAndCheckResp(t, zki)
	}

	log.Debug("block:0 batch:7")
	// simulate the PoolL2Txs of the batch6
	batchPoolL2 := `
	Type: PoolL2
	PoolTransferToEthAddr(1) A-B: 200 (126)
	PoolTransferToEthAddr(0) B-C: 100 (126)`
	l2Txs, err := tc.GeneratePoolL2Txs(batchPoolL2)
	require.NoError(t, err)
	// add AccountCreationAuths that will be used at the next batch
	_ = addAccCreationAuth(t, tc, l2DBTxSel, ChainID, hermezContractAddr, "B")
	_ = addAccCreationAuth(t, tc, l2DBTxSel, ChainID, hermezContractAddr, "C")
	addL2Txs(t, l2DBTxSel, l2Txs) // Add L2s to TxSelector.L2DB
	l1UserTxs := til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[6].Batch.ForgeL1TxsNum])
	// TxSelector select the transactions for the next Batch
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, discardedL2Txs, err :=
		txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
	require.NoError(t, err)
	// BatchBuilder build Batch
	zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t,
		"4392049343656836675348565048374261353937130287163762821533580216441778455298",
		bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
	err = l2DBTxSel.StartForging(common.TxIDsFromPoolL2Txs(oL2Txs),
		txsel.LocalAccountsDB().CurrentBatch())
	require.NoError(t, err)
	var batchNum common.BatchNum
	err = l2DBTxSel.UpdateTxsInfo(discardedL2Txs, batchNum)
	require.NoError(t, err)

	log.Debug("block:0 batch:8")
	// simulate the PoolL2Txs of the batch8
	batchPoolL2 = `
	Type: PoolL2
	PoolTransfer(0) A-B: 100 (126)
	PoolTransfer(0) C-A: 50 (126)
	PoolTransfer(1) B-C: 100 (126)
	PoolExit(0) A: 100 (126)`
	l2Txs, err = tc.GeneratePoolL2Txs(batchPoolL2)
	require.NoError(t, err)
	addL2Txs(t, l2DBTxSel, l2Txs) // Add L2s to TxSelector.L2DB
	l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[7].Batch.ForgeL1TxsNum])
	// TxSelector select the transactions for the next Batch
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, discardedL2Txs, err =
		txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
	require.NoError(t, err)
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t,
		"8905191229562583213069132470917469035834300549892959854483573322676101624713",
		bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
	err = l2DBTxSel.StartForging(common.TxIDsFromPoolL2Txs(l2Txs),
		txsel.LocalAccountsDB().CurrentBatch())
	require.NoError(t, err)
	err = l2DBTxSel.UpdateTxsInfo(discardedL2Txs, batchNum)
	require.NoError(t, err)

	log.Debug("(batch9) block:1 batch:1")
	// simulate the PoolL2Txs of the batch9
	batchPoolL2 = `
	Type: PoolL2
	PoolTransfer(0) D-A: 300 (126)
	PoolTransfer(0) B-D: 100 (126)`
	l2Txs, err = tc.GeneratePoolL2Txs(batchPoolL2)
	require.NoError(t, err)
	addL2Txs(t, l2DBTxSel, l2Txs) // Add L2s to TxSelector.L2DB
	l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[1].Rollup.Batches[0].Batch.ForgeL1TxsNum])
	// TxSelector select the transactions for the next Batch
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, discardedL2Txs, err =
		txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
	require.NoError(t, err)
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t,
		"20593679664586247774284790801579542411781976279024409415159440382607791042723",
		bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
	err = l2DBTxSel.StartForging(common.TxIDsFromPoolL2Txs(l2Txs),
		txsel.LocalAccountsDB().CurrentBatch())
	require.NoError(t, err)
	err = l2DBTxSel.UpdateTxsInfo(discardedL2Txs, batchNum)
	require.NoError(t, err)

	log.Debug("(batch10) block:1 batch:2")
	l2Txs = []common.PoolL2Tx{}
	l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[1].Rollup.Batches[1].Batch.ForgeL1TxsNum])
	// TxSelector select the transactions for the next Batch
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, discardedL2Txs, err =
		txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
	require.NoError(t, err)
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	// same root as previous batch, as the L1CoordinatorTxs created by the
	// Til set is not created by the TxSelector in this test
	assert.Equal(t,
		"20593679664586247774284790801579542411781976279024409415159440382607791042723",
		bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
	err = l2DBTxSel.StartForging(common.TxIDsFromPoolL2Txs(l2Txs),
		txsel.LocalAccountsDB().CurrentBatch())
	require.NoError(t, err)
	err = l2DBTxSel.UpdateTxsInfo(discardedL2Txs, batchNum)
	require.NoError(t, err)

	bb.LocalStateDB().Close()
	txsel.LocalAccountsDB().Close()
	syncStateDB.Close()
}

// TestZKInputsExitWithFee0 checks the case where there is a PoolTxs of type
// Exit with fee 0 for a TokenID that the Coordinator does not have it
// registered yet
func TestZKInputsExitWithFee0(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)

	var set = `
	Type: Blockchain
	AddToken(1)

	CreateAccountDeposit(1) A: 1000
	CreateAccountDeposit(1) B: 1000
	CreateAccountDeposit(1) C: 1000
	> batchL1
	> batchL1

	CreateAccountCoordinator(1) Coord
	> batch
	> block
	`
	blocks, err := tc.GenerateBlocks(set)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["Coord"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, NLevels)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()
	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		TxProcessorConfig: txprocConfig,
	}

	// batch2
	// TxSelector select the transactions for the next Batch
	l1UserTxs := til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[1].Batch.ForgeL1TxsNum])
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
		txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
	require.NoError(t, err)
	// BatchBuilder build Batch
	zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t,
		"3050252508378236752695438107925920517579600844238792454632938959089837319058",
		bb.LocalStateDB().MT.Root().BigInt().String())
	h, err := zki.HashGlobalData()
	require.NoError(t, err)
	assert.Equal(t,
		"136173330006576039857485697813777018179965431269591881328654192642028135989",
		h.String())
	sendProofAndCheckResp(t, zki)

	// batch3
	batchPoolL2 := `
	Type: PoolL2
	PoolExit(1) A: 100 (0)`
	l2Txs, err := tc.GeneratePoolL2Txs(batchPoolL2)
	require.NoError(t, err)
	addL2Txs(t, l2DBTxSel, l2Txs) // Add L2s to TxSelector.L2DB
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, discardedL2Txs, err :=
		txsel.GetL1L2TxSelection(txprocConfig, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, len(coordIdxs))
	assert.Equal(t, 0, len(oL1UserTxs))
	assert.Equal(t, 1, len(oL1CoordTxs))
	assert.Equal(t, 1, len(oL2Txs))
	assert.Equal(t, 0, len(discardedL2Txs))
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t,
		"2941150582529643425331223235752941075548157545257982041291886277157404095484",
		bb.LocalStateDB().MT.Root().BigInt().String())
	h, err = zki.HashGlobalData()
	require.NoError(t, err)
	assert.Equal(t,
		"11526955144859107275861838429358092025337347677758832533226842081116224550335",
		h.String())
	assert.Equal(t, common.EthAddrToBigInt(tc.Users["Coord"].Addr), zki.EthAddr3[0])
	assert.Equal(t, "0", zki.EthAddr3[1].String())
	sendProofAndCheckResp(t, zki)

	bb.LocalStateDB().Close()
	txsel.LocalAccountsDB().Close()
	syncStateDB.Close()
}

// TestZKInputsAtomicTxs checks the zki for a basic flow using atomic txs
func TestZKInputsAtomicTxs(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)
	// generate test transactions, the L1CoordinatorTxs generated by Til
	// will be ignored at this test, as will be the TxSelector who
	// generates them when needed
	blocks, err := tc.GenerateBlocks(`
	Type: Blockchain
	> batch
	CreateAccountDeposit(0) A: 500
	CreateAccountDeposit(0) B: 300
	> batchL1
	> batchL1
	> batchL1
	> block
	`)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["A"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, 16)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()

	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		TxProcessorConfig: txprocessor.Config{
			NLevels:  16,
			MaxFeeTx: 2, // Different from CC
			MaxL1Tx:  2,
			MaxTx:    3,
			ChainID:  ChainID,
		},
	}

	// loop over the first 6 batches
	expectedRoots := []string{
		"0",
		"0",
		"4699107814499591308646397797397227709064109108834870471069309801839156153817",
		"13944326923512084386654700005952881304026772608660229852183663245455287011418",
	}
	// Process 3 first batches using til
	for i := 0; i < 3; i++ {
		log.Debugf("block:0 batch:%d", i+1)
		var l1UserTxs []common.L1Tx
		if blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum != nil {
			l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum])
		}
		// TxSelector select the transactions for the next Batch
		coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
			txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
		require.NoError(t, err)
		// BatchBuilder build Batch
		zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
		require.NoError(t, err)
		assert.Equal(t, expectedRoots[i], bb.LocalStateDB().MT.Root().BigInt().String())
		sendProofAndCheckResp(t, zki)
	}
	// Manually generate atomic txs
	atomicTxA := common.PoolL2Tx{
		FromIdx:       256,
		ToIdx:         257,
		TokenID:       0,
		Amount:        big.NewInt(100),
		Fee:           0,
		Nonce:         0,
		RqFromIdx:     257,
		RqToIdx:       256,
		RqTokenID:     0,
		RqAmount:      big.NewInt(50),
		RqFee:         0,
		RqNonce:       0,
		State:         common.PoolL2TxStatePending,
		RqOffset:      1,
		AtomicGroupID: common.AtomicGroupID([common.AtomicGroupIDLen]byte{1}),
	}
	_, err = common.NewPoolL2Tx(&atomicTxA)
	require.NoError(t, err)
	aWallet := til.NewUser(1, "A")
	hashTxA, err := atomicTxA.HashToSign(ChainID)
	require.NoError(t, err)
	atomicTxA.Signature = aWallet.BJJ.SignPoseidon(hashTxA).Compress()
	atomicTxB := common.PoolL2Tx{
		FromIdx:       257,
		ToIdx:         256,
		TokenID:       0,
		Amount:        big.NewInt(50),
		Fee:           0,
		Nonce:         0,
		RqFromIdx:     256,
		RqToIdx:       257,
		RqTokenID:     0,
		RqAmount:      big.NewInt(100),
		RqFee:         0,
		RqNonce:       0,
		State:         common.PoolL2TxStatePending,
		RqOffset:      7,
		AtomicGroupID: common.AtomicGroupID([common.AtomicGroupIDLen]byte{1}),
	}
	_, err = common.NewPoolL2Tx(&atomicTxB)
	require.NoError(t, err)
	bWallet := til.NewUser(2, "B")
	hashTxB, err := atomicTxB.HashToSign(ChainID)
	require.NoError(t, err)
	atomicTxB.Signature = bWallet.BJJ.SignPoseidon(hashTxB).Compress()

	// Add txs to DB
	require.NoError(t, l2DBTxSel.AddTxTest(&atomicTxA))
	require.NoError(t, l2DBTxSel.AddTxTest(&atomicTxB))

	// TxSelector select the transactions for the next Batch
	coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
		txsel.GetL1L2TxSelection(txprocConfig, nil, nil)
	require.NoError(t, err)

	// BatchBuilder build Batch
	zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[3], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
}

func TestZKInputsAtomicTxs2(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)
	// generate test transactions, the L1CoordinatorTxs generated by Til
	// will be ignored at this test, as will be the TxSelector who
	// generates them when needed
	blocks, err := tc.GenerateBlocks(`
	Type: Blockchain
	> batch
	CreateAccountDeposit(0) A: 500
	CreateAccountDeposit(0) B: 300
	> batchL1
	> batchL1
	> batchL1
	> block
	`)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["A"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, 16)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()

	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		TxProcessorConfig: txprocessor.Config{
			NLevels:  16,
			MaxFeeTx: 2,
			MaxL1Tx:  2,
			MaxTx:    3,
			ChainID:  ChainID,
		},
	}

	// loop over the first 6 batches
	expectedRoots := []string{
		"0",
		"0",
		"4699107814499591308646397797397227709064109108834870471069309801839156153817",
		"14308891764306680383563176140366979045110288310078804500780349771465245905349",
		"10888624449169052621639761533901677350089347579730136443445410532938653329960",
		"10888624449169052621639761533901677350089347579730136443445410532938653329960",
		"16211152882311374033982856603217667819334355283282393300267321154116196396852",
	}
	// Process 3 first batches using til (batch 0 to batch 2)
	for i := 0; i < 3; i++ {
		log.Debugf("block:0 batch:%d", i+1)
		var l1UserTxs []common.L1Tx
		if blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum != nil {
			l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum])
		}
		// TxSelector select the transactions for the next Batch
		coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
			txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
		require.NoError(t, err)
		// BatchBuilder build Batch
		zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
		require.NoError(t, err)
		assert.Equal(t, expectedRoots[i], bb.LocalStateDB().MT.Root().BigInt().String())
		sendProofAndCheckResp(t, zki)
	}
	// Batch 3
	// Manually generate atomic txs
	// TX 1
	aWallet := til.NewUser(1, "A")
	bWallet := til.NewUser(2, "B")
	cWallet := til.NewUser(3, "C")
	dWallet := til.NewUser(4, "D")
	tx1 := common.PoolL2Tx{
		FromIdx:       256,
		ToEthAddr:     bWallet.Addr,
		Amount:        big.NewInt(100),
		RqOffset:      2,
		RqAmount:      big.NewInt(50),
		RqFromIdx:     257,
		RqToEthAddr:   aWallet.Addr,
		AtomicGroupID: common.AtomicGroupID([32]byte{1}),
	}
	_, err = common.NewPoolL2Tx(&tx1)
	require.NoError(t, err)
	hashTx1, err := tx1.HashToSign(ChainID)
	require.NoError(t, err)
	tx1.Signature = aWallet.BJJ.SignPoseidon(hashTx1).Compress()
	// TX 2
	tx2 := common.PoolL2Tx{
		FromIdx: 256,
		ToIdx:   257,
		Amount:  big.NewInt(50),
		Nonce:   1,
		State:   common.PoolL2TxStatePending,
	}
	_, err = common.NewPoolL2Tx(&tx2)
	require.NoError(t, err)
	hashTx2, err := tx2.HashToSign(ChainID)
	require.NoError(t, err)
	tx2.Signature = aWallet.BJJ.SignPoseidon(hashTx2).Compress()
	// TX 3
	tx3 := common.PoolL2Tx{
		FromIdx:       257,
		ToEthAddr:     aWallet.Addr,
		Amount:        big.NewInt(50),
		RqFromIdx:     256,
		RqToEthAddr:   bWallet.Addr,
		RqAmount:      big.NewInt(100),
		State:         common.PoolL2TxStatePending,
		RqOffset:      6,
		AtomicGroupID: common.AtomicGroupID([32]byte{1}),
	}
	_, err = common.NewPoolL2Tx(&tx3)
	require.NoError(t, err)
	hashTx3, err := tx3.HashToSign(ChainID)
	require.NoError(t, err)
	tx3.Signature = bWallet.BJJ.SignPoseidon(hashTx3).Compress()
	// BatchBuilder build Batch
	zki, err := bb.BuildBatch(nil, configBatch, nil, nil, []common.PoolL2Tx{tx1, tx2, tx3})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[3], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 4
	tx4 := common.L1Tx{
		FromEthAddr:   common.FFAddr,
		FromBJJ:       cWallet.BJJ.Public().Compress(),
		TokenID:       0,
		FromIdx:       0,
		ToIdx:         0,
		Amount:        big.NewInt(0),
		DepositAmount: big.NewInt(0),
		UserOrigin:    false,
		Type:          common.TxTypeCreateAccountDeposit,
	}
	tx5 := common.L1Tx{
		FromEthAddr:   common.FFAddr,
		FromBJJ:       dWallet.BJJ.Public().Compress(),
		TokenID:       0,
		FromIdx:       0,
		ToIdx:         0,
		Amount:        big.NewInt(0),
		DepositAmount: big.NewInt(0),
		UserOrigin:    false,
		Type:          common.TxTypeCreateAccountDeposit,
	}
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(nil, configBatch, nil, []common.L1Tx{tx4, tx5}, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[4], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 5 (empty)
	zki, err = bb.BuildBatch(nil, configBatch, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[5], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 6
	// TX 6
	tx6 := common.PoolL2Tx{
		FromIdx:       256,
		ToEthAddr:     common.FFAddr,
		ToBJJ:         cWallet.BJJ.Public().Compress(),
		Amount:        big.NewInt(100),
		Nonce:         2,
		RqFromIdx:     257,
		RqToEthAddr:   common.FFAddr,
		RqToBJJ:       dWallet.BJJ.Public().Compress(),
		RqAmount:      big.NewInt(50),
		RqNonce:       1,
		RqOffset:      1,
		AtomicGroupID: common.AtomicGroupID([32]byte{2}),
	}
	_, err = common.NewPoolL2Tx(&tx6)
	require.NoError(t, err)
	hashTx6, err := tx6.HashToSign(ChainID)
	require.NoError(t, err)
	tx6.Signature = aWallet.BJJ.SignPoseidon(hashTx6).Compress()
	// TX 7
	tx7 := common.PoolL2Tx{
		FromIdx:       257,
		ToEthAddr:     common.FFAddr,
		ToBJJ:         dWallet.BJJ.Public().Compress(),
		Amount:        big.NewInt(50),
		Nonce:         1,
		RqFromIdx:     256,
		RqToEthAddr:   common.FFAddr,
		RqToBJJ:       cWallet.BJJ.Public().Compress(),
		RqAmount:      big.NewInt(100),
		RqNonce:       2,
		RqOffset:      7,
		AtomicGroupID: common.AtomicGroupID([32]byte{2}),
	}
	_, err = common.NewPoolL2Tx(&tx7)
	require.NoError(t, err)
	hashTx7, err := tx7.HashToSign(ChainID)
	require.NoError(t, err)
	tx7.Signature = bWallet.BJJ.SignPoseidon(hashTx7).Compress()
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch(nil, configBatch, nil, nil, []common.PoolL2Tx{tx6, tx7})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[6], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
}

func TestZKInputsAtomicTxsEdge(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)
	// generate test transactions, the L1CoordinatorTxs generated by Til
	// will be ignored at this test, as will be the TxSelector who
	// generates them when needed
	blocks, err := tc.GenerateBlocks(`
	Type: Blockchain
	> batch
	CreateAccountDeposit(0) A: 500
	CreateAccountDeposit(0) B: 300
	> batchL1
	> batchL1
	> batchL1
	> block
	`)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["A"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, 16)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()

	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		TxProcessorConfig: txprocessor.Config{
			NLevels:  16,
			MaxFeeTx: 2,
			MaxL1Tx:  2,
			MaxTx:    5,
			ChainID:  ChainID,
		},
	}

	expectedRoots := []string{
		"0",
		"0",
		"4699107814499591308646397797397227709064109108834870471069309801839156153817",
		"8648946464699997298087254217875026532493805010600455501624011406286390714724",
		"7684290783592306799291180752833613052466875765217001615178964094222347541153",
		"17470597030213466849471229074732312786496599837367040991361767144166199600072",
		"9314847606205893808530649466856518575089682244658827420995223242741746163552",
	}

	// Process 3 first batches using til (batch 0 to batch 2)
	for i := 0; i < 3; i++ {
		log.Debugf("block:0 batch:%d", i+1)
		var l1UserTxs []common.L1Tx
		if blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum != nil {
			l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum])
		}
		// TxSelector select the transactions for the next Batch
		coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
			txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
		require.NoError(t, err)
		// BatchBuilder build Batch
		zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
		require.NoError(t, err)
		assert.Equal(t, expectedRoots[i], bb.LocalStateDB().MT.Root().BigInt().String())
		sendProofAndCheckResp(t, zki)
	}
	// Batch 3
	// Manually generate atomic txs
	// TX 1
	aWallet := til.NewUser(1, "A")
	bWallet := til.NewUser(2, "B")
	tx1 := common.PoolL2Tx{
		FromIdx:   256,
		ToIdx:     257,
		Amount:    big.NewInt(0),
		Fee:       126,
		RqOffset:  1,
		RqAmount:  big.NewInt(10),
		RqFromIdx: 257,
		RqToIdx:   1,
		RqFee:     126,
	}
	_, err = common.NewPoolL2Tx(&tx1)
	require.NoError(t, err)
	hashTx1, err := tx1.HashToSign(ChainID)
	require.NoError(t, err)
	tx1.Signature = aWallet.BJJ.SignPoseidon(hashTx1).Compress()
	// TX 2
	tx2 := common.PoolL2Tx{
		FromIdx:   257,
		ToIdx:     1,
		Amount:    big.NewInt(10),
		Fee:       126,
		RqOffset:  7,
		RqAmount:  big.NewInt(0),
		RqFromIdx: 256,
		RqToIdx:   257,
		RqFee:     126,
		Type:      common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx2)
	require.NoError(t, err)
	hashTx2, err := tx2.HashToSign(ChainID)
	require.NoError(t, err)
	tx2.Signature = bWallet.BJJ.SignPoseidon(hashTx2).Compress()
	// BatchBuilder build Batch
	zki, err := bb.BuildBatch([]common.Idx{256}, configBatch, nil, nil, []common.PoolL2Tx{tx1, tx2})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[3], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 4
	// TX3
	tx3 := common.PoolL2Tx{
		FromIdx:   256,
		ToIdx:     257,
		Amount:    big.NewInt(20),
		Nonce:     1,
		Fee:       127,
		RqOffset:  3,
		RqAmount:  big.NewInt(0),
		RqFromIdx: 256,
		RqToIdx:   257,
		RqNonce:   2,
		RqFee:     120,
	}
	_, err = common.NewPoolL2Tx(&tx3)
	require.NoError(t, err)
	hashTx3, err := tx3.HashToSign(ChainID)
	require.NoError(t, err)
	tx3.Signature = aWallet.BJJ.SignPoseidon(hashTx3).Compress()
	// TX4
	tx4 := common.PoolL2Tx{
		FromIdx:   257,
		ToIdx:     256,
		Amount:    big.NewInt(0),
		Nonce:     1,
		Fee:       128,
		RqOffset:  1,
		RqAmount:  big.NewInt(20),
		RqFromIdx: 257,
		RqToIdx:   256,
		RqNonce:   2,
		RqFee:     125,
	}
	_, err = common.NewPoolL2Tx(&tx4)
	require.NoError(t, err)
	hashTx4, err := tx4.HashToSign(ChainID)
	require.NoError(t, err)
	tx4.Signature = bWallet.BJJ.SignPoseidon(hashTx4).Compress()
	// TX5
	tx5 := common.PoolL2Tx{
		FromIdx:   257,
		ToIdx:     256,
		Amount:    big.NewInt(20),
		Nonce:     2,
		Fee:       125,
		RqOffset:  7,
		RqAmount:  big.NewInt(0),
		RqFromIdx: 257,
		RqToIdx:   256,
		RqNonce:   1,
		RqFee:     128,
	}
	_, err = common.NewPoolL2Tx(&tx5)
	require.NoError(t, err)
	hashTx5, err := tx5.HashToSign(ChainID)
	require.NoError(t, err)
	tx5.Signature = bWallet.BJJ.SignPoseidon(hashTx5).Compress()
	// TX6
	tx6 := common.PoolL2Tx{
		FromIdx:   256,
		ToIdx:     257,
		Amount:    big.NewInt(0),
		Nonce:     2,
		Fee:       120,
		RqOffset:  5,
		RqAmount:  big.NewInt(20),
		RqFromIdx: 256,
		RqToIdx:   257,
		RqNonce:   1,
		RqFee:     127,
	}
	_, err = common.NewPoolL2Tx(&tx6)
	require.NoError(t, err)
	hashTx6, err := tx6.HashToSign(ChainID)
	require.NoError(t, err)
	tx6.Signature = aWallet.BJJ.SignPoseidon(hashTx6).Compress()
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch([]common.Idx{256}, configBatch, nil, nil, []common.PoolL2Tx{tx3, tx4, tx5, tx6})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[4], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 5
	// TX7
	tx7 := common.PoolL2Tx{
		FromIdx: 256,
		ToIdx:   1,
		Amount:  big.NewInt(10),
		Nonce:   3,
		Fee:     121,
		Type:    common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx7)
	require.NoError(t, err)
	hashTx7, err := tx7.HashToSign(ChainID)
	require.NoError(t, err)
	tx7.Signature = aWallet.BJJ.SignPoseidon(hashTx7).Compress()
	// TX8
	tx8 := common.PoolL2Tx{
		FromIdx: 257,
		ToIdx:   1,
		Amount:  big.NewInt(10),
		Nonce:   3,
		Fee:     122,
		Type:    common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx8)
	require.NoError(t, err)
	hashTx8, err := tx8.HashToSign(ChainID)
	require.NoError(t, err)
	tx8.Signature = bWallet.BJJ.SignPoseidon(hashTx8).Compress()
	// TX9
	tx9 := common.PoolL2Tx{
		FromIdx: 256,
		ToIdx:   1,
		Amount:  big.NewInt(10),
		Nonce:   4,
		Fee:     123,
		Type:    common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx9)
	require.NoError(t, err)
	hashTx9, err := tx9.HashToSign(ChainID)
	require.NoError(t, err)
	tx9.Signature = aWallet.BJJ.SignPoseidon(hashTx9).Compress()
	// TX10
	tx10 := common.PoolL2Tx{
		FromIdx: 257,
		ToIdx:   1,
		Amount:  big.NewInt(10),
		Nonce:   4,
		Fee:     124,
		Type:    common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx10)
	require.NoError(t, err)
	hashTx10, err := tx10.HashToSign(ChainID)
	require.NoError(t, err)
	tx10.Signature = bWallet.BJJ.SignPoseidon(hashTx10).Compress()
	// TX11
	tx11 := common.PoolL2Tx{
		FromIdx:   256,
		ToIdx:     1,
		Amount:    big.NewInt(10),
		Nonce:     5,
		Fee:       125,
		RqFromIdx: 256,
		RqToIdx:   1,
		RqAmount:  big.NewInt(10),
		RqNonce:   3,
		RqFee:     121,
		RqOffset:  4,
		Type:      common.TxTypeExit,
	}
	_, err = common.NewPoolL2Tx(&tx11)
	require.NoError(t, err)
	hashTx11, err := tx11.HashToSign(ChainID)
	require.NoError(t, err)
	tx11.Signature = aWallet.BJJ.SignPoseidon(hashTx11).Compress()

	zki, err = bb.BuildBatch([]common.Idx{256}, configBatch, nil, nil, []common.PoolL2Tx{tx7, tx8, tx9, tx10, tx11})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[5], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)

	// Batch 6
	// TX 12
	tx12 := common.PoolL2Tx{
		FromIdx:   256,
		ToIdx:     257,
		Amount:    big.NewInt(50),
		Nonce:     6,
		Fee:       130,
		RqFromIdx: 257,
		RqToIdx:   1,
		RqAmount:  big.NewInt(50),
		RqNonce:   5,
		RqOffset:  1,
		RqFee:     128,
	}
	_, err = common.NewPoolL2Tx(&tx12)
	require.NoError(t, err)
	hashTx12, err := tx12.HashToSign(ChainID)
	require.NoError(t, err)
	tx12.Signature = aWallet.BJJ.SignPoseidon(hashTx12).Compress()
	// TX 13
	tx13 := common.PoolL2Tx{
		FromIdx:   257,
		ToIdx:     1,
		Amount:    big.NewInt(50),
		Nonce:     5,
		Fee:       128,
		RqFromIdx: 256,
		RqToIdx:   257,
		RqAmount:  big.NewInt(50),
		RqNonce:   6,
		RqOffset:  7,
		RqFee:     130,
	}
	_, err = common.NewPoolL2Tx(&tx13)
	require.NoError(t, err)
	hashTx13, err := tx13.HashToSign(ChainID)
	require.NoError(t, err)
	tx13.Signature = bWallet.BJJ.SignPoseidon(hashTx13).Compress()
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch([]common.Idx{256}, configBatch, nil, nil, []common.PoolL2Tx{tx12, tx13})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[6], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
}

func TestZKInputsMaxNumBatch(t *testing.T) {
	tc := til.NewContext(ChainID, common.RollupConstMaxL1UserTx)
	// generate test transactions, the L1CoordinatorTxs generated by Til
	// will be ignored at this test, as will be the TxSelector who
	// generates them when needed
	blocks, err := tc.GenerateBlocks(`
	Type: Blockchain
	> batch
	CreateAccountDeposit(0) A: 500
	CreateAccountDeposit(0) B: 300
	> batchL1
	> batchL1
	> batchL1
	> block
	`)
	require.NoError(t, err)

	hermezContractAddr := ethCommon.HexToAddress("0xc344E203a046Da13b0B4467EB7B3629D0C99F6E6")
	txsel, l2DBTxSel, syncStateDB := initTxSelector(t, ChainID, hermezContractAddr, tc.Users["A"])

	bbDir, err := ioutil.TempDir("", "tmpBatchBuilderDB")
	require.NoError(t, err)
	deleteme = append(deleteme, bbDir)
	bb, err := batchbuilder.NewBatchBuilder(bbDir, syncStateDB, 0, 16)
	require.NoError(t, err)

	// restart nonces of TilContext, as will be set by generating directly
	// the PoolL2Txs for each specific batch with tc.GeneratePoolL2Txs
	tc.RestartNonces()

	// add tokens to HistoryDB to avoid breaking FK constrains
	addTokens(t, tc, l2DBTxSel.DB())

	configBatch := &batchbuilder.ConfigBatch{
		TxProcessorConfig: txprocessor.Config{
			NLevels:  16,
			MaxFeeTx: 2,
			MaxL1Tx:  2,
			MaxTx:    3,
			ChainID:  ChainID,
		},
	}

	// loop over the first 6 batches
	expectedRoots := []string{
		"0",
		"0",
		"4699107814499591308646397797397227709064109108834870471069309801839156153817",
		"14294611316400822660176403598744006193014267799947398151388212959007124022702",
		"21860840608558568469163210346771935134727995692557954944916248531843343259626",
	}
	// Process 3 first batches using til (batch 0 to batch 2)
	for i := 0; i < 3; i++ {
		log.Debugf("block:0 batch:%d", i+1)
		var l1UserTxs []common.L1Tx
		if blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum != nil {
			l1UserTxs = til.L1TxsToCommonL1Txs(tc.Queues[*blocks[0].Rollup.Batches[i].Batch.ForgeL1TxsNum])
		}
		// TxSelector select the transactions for the next Batch
		coordIdxs, _, oL1UserTxs, oL1CoordTxs, oL2Txs, _, err :=
			txsel.GetL1L2TxSelection(txprocConfig, l1UserTxs, nil)
		require.NoError(t, err)
		// BatchBuilder build Batch
		zki, err := bb.BuildBatch(coordIdxs, configBatch, oL1UserTxs, oL1CoordTxs, oL2Txs)
		require.NoError(t, err)
		assert.Equal(t, expectedRoots[i], bb.LocalStateDB().MT.Root().BigInt().String())
		sendProofAndCheckResp(t, zki)
	}
	aWallet := til.NewUser(1, "A")
	bWallet := til.NewUser(2, "B")
	cWallet := til.NewUser(3, "C")
	// Batch 3
	// Manually generate atomic txs
	// Tx 0
	tx0 := common.L1Tx{
		FromEthAddr:   common.FFAddr,
		FromBJJ:       cWallet.BJJ.Public().Compress(),
		DepositAmount: big.NewInt(0),
		Amount:        big.NewInt(0),
		UserOrigin:    false,
		Type:          common.TxTypeCreateAccountDeposit,
	}
	// TX 1
	tx1 := common.PoolL2Tx{
		FromIdx:     256,
		ToEthAddr:   bWallet.Addr,
		Amount:      big.NewInt(100),
		Fee:         180,
		MaxNumBatch: 4,
	}
	_, err = common.NewPoolL2Tx(&tx1)
	require.NoError(t, err)
	hashTx1, err := tx1.HashToSign(ChainID)
	require.NoError(t, err)
	tx1.Signature = aWallet.BJJ.SignPoseidon(hashTx1).Compress()
	// BatchBuilder build Batch
	zki, err := bb.BuildBatch([]common.Idx{257}, configBatch, nil, []common.L1Tx{tx0}, []common.PoolL2Tx{tx1})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[3], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
	// Batch 4
	// TX 2
	tx2 := common.PoolL2Tx{
		FromIdx:     257,
		ToEthAddr:   common.FFAddr,
		ToBJJ:       cWallet.BJJ.Public().Compress(),
		Amount:      big.NewInt(55),
		Fee:         126,
		MaxNumBatch: 21,
	}
	_, err = common.NewPoolL2Tx(&tx2)
	require.NoError(t, err)
	hashTx2, err := tx2.HashToSign(ChainID)
	require.NoError(t, err)
	tx2.Signature = bWallet.BJJ.SignPoseidon(hashTx2).Compress()
	// TX 3
	tx3 := common.PoolL2Tx{
		FromIdx:     258,
		ToIdx:       256,
		Amount:      big.NewInt(30),
		Fee:         125,
		MaxNumBatch: 9,
	}
	_, err = common.NewPoolL2Tx(&tx3)
	require.NoError(t, err)
	hashTx3, err := tx3.HashToSign(ChainID)
	require.NoError(t, err)
	tx3.Signature = cWallet.BJJ.SignPoseidon(hashTx3).Compress()
	// TX 4
	tx4 := common.PoolL2Tx{
		FromIdx:     256,
		ToIdx:       1,
		Amount:      big.NewInt(20),
		Nonce:       1,
		Fee:         121,
		MaxNumBatch: 5,
	}
	_, err = common.NewPoolL2Tx(&tx4)
	require.NoError(t, err)
	hashTx4, err := tx4.HashToSign(ChainID)
	require.NoError(t, err)
	tx4.Signature = aWallet.BJJ.SignPoseidon(hashTx4).Compress()
	// BatchBuilder build Batch
	zki, err = bb.BuildBatch([]common.Idx{256}, configBatch, nil, nil, []common.PoolL2Tx{tx2, tx3, tx4})
	require.NoError(t, err)
	assert.Equal(t, expectedRoots[4], bb.LocalStateDB().MT.Root().BigInt().String())
	sendProofAndCheckResp(t, zki)
}
