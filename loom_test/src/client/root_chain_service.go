package client

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ningxin18/plasma_cash/client/plasma_cash"
	"github.com/ningxin18/plasma_cash/client/plasma_cash/eth/ethcontract"
)

type RootChainService struct {
	Name           string
	plasmaContract *ethcontract.RootChain
	callerKey      *ecdsa.PrivateKey
	callerAddr     common.Address
	transactOpts   *bind.TransactOpts
	callOpts       *bind.CallOpts
}

func (d *RootChainService) PlasmaCoin(slot uint64) (*plasma_cash.PlasmaCoin, error) {
	uid, depositBlockNum, denom, ownerAddr, state, mode, contractAddr, err := d.plasmaContract.GetPlasmaCoin(
		&bind.CallOpts{From: d.callerAddr},
		slot,
	)
	if err != nil {
		return nil, err
	}
	return &plasma_cash.PlasmaCoin{
		UID:             uid,
		DepositBlockNum: depositBlockNum,
		Denomination:    denom,
		Owner:           ownerAddr.Hex(),
		State:           plasma_cash.PlasmaCoinState(state),
		Mode:            plasma_cash.PlasmaCoinMode(mode),
		ContractAddress: contractAddr.Hex(),
	}, nil
}

func (d *RootChainService) Withdraw(slot uint64) error {
	_, err := d.plasmaContract.Withdraw(d.transactOpts, slot)
	return err
}

func (d *RootChainService) ChallengeBefore(slot uint64, prevTx plasma_cash.Tx, exitingTx plasma_cash.Tx,
	prevTxInclusionProof plasma_cash.Proof, exitingTxInclusionProof plasma_cash.Proof,
	sig []byte, prevTxBlockNum *big.Int, exitingTxBlockNum *big.Int) ([]byte, error) {
	var err error
	var prevTxBytes []byte
	if prevTx != nil {
		prevTxBytes, err = prevTx.RlpEncode()
		if err != nil {
			return nil, err
		}
	}
	exitingTxBytes, err := exitingTx.RlpEncode()
	if err != nil {
		return nil, err
	}

	d.transactOpts.Value = big.NewInt(100000000000000000) //0.1 eth, TODO make the bond configurable
	exitblocks := [2]*big.Int{prevTxBlockNum, exitingTxBlockNum}
	tx, err := d.plasmaContract.ChallengeBefore(
		d.transactOpts, slot, prevTxBytes, exitingTxBytes,
		prevTxInclusionProof, exitingTxInclusionProof, sig,
		exitblocks)
	d.transactOpts.Value = big.NewInt(0)
	if err != nil {
		return nil, err
	}
	return tx.Hash().Bytes(), nil
}

func (d *RootChainService) RespondChallengeBefore(slot uint64, challengingTxHash [32]byte, respondingBlockNumber *big.Int,
	respondingTx plasma_cash.Tx, proof plasma_cash.Proof, sig []byte) ([]byte, error) {
	respondingTxBytes, err := respondingTx.RlpEncode()
	if err != nil {
		return nil, err
	}
	tx, err := d.plasmaContract.RespondChallengeBefore(
		d.transactOpts, slot, challengingTxHash, respondingBlockNumber, respondingTxBytes, proof, sig)
	if err != nil {
		return nil, err
	}
	return tx.Hash().Bytes(), nil
}

func (d *RootChainService) ChallengeBetween(slot uint64, challengingBlockNumber *big.Int,
	challengingTx plasma_cash.Tx, proof plasma_cash.Proof, sig []byte) ([]byte, error) {

	challengingTxBytes, err := challengingTx.RlpEncode()
	if err != nil {
		return nil, err
	}
	tx, err := d.plasmaContract.ChallengeBetween(
		d.transactOpts, slot, challengingBlockNumber, challengingTxBytes, proof, sig)
	if err != nil {
		return nil, err
	}
	return tx.Hash().Bytes(), nil
}

func (d *RootChainService) ChallengeAfter(slot uint64, challengingBlockNumber *big.Int,
	challengingTx plasma_cash.Tx, proof plasma_cash.Proof, sig []byte) ([]byte, error) {

	challengingTxBytes, err := challengingTx.RlpEncode()
	if err != nil {
		return nil, err
	}
	tx, err := d.plasmaContract.ChallengeAfter(
		d.transactOpts, slot, challengingBlockNumber, challengingTxBytes, proof, sig)
	if err != nil {
		return nil, err
	}
	return tx.Hash().Bytes(), nil
}

func (d *RootChainService) StartExit(
	slot uint64, prevTx plasma_cash.Tx, exitingTx plasma_cash.Tx, prevTxInclusion plasma_cash.Proof, exitingTxInclusion plasma_cash.Proof,
	sigs []byte, prevTxIncBlock *big.Int, exitingTxIncBlock *big.Int) ([]byte, error) {

	var prevTxBytes []byte
	var err error
	if prevTx != nil {
		prevTxBytes, err = prevTx.RlpEncode()
		if err != nil {
			return nil, err
		}
	}

	exitingTxBytes, err := exitingTx.RlpEncode()
	if err != nil {
		return nil, err
	}
	d.transactOpts.Value = big.NewInt(100000000000000000) //0.1 eth, TODO make the bond configurable

	exitblocks := [2]*big.Int{prevTxIncBlock, exitingTxIncBlock}
	tx, err := d.plasmaContract.StartExit(
		d.transactOpts, slot,
		prevTxBytes, exitingTxBytes, prevTxInclusion, exitingTxInclusion,
		sigs, exitblocks)

	d.transactOpts.Value = big.NewInt(0)
	if err != nil {
		return nil, err
	}
	return tx.Hash().Bytes(), nil
}

func (d *RootChainService) CancelExit(slot uint64) error {
	_, err := d.plasmaContract.CancelExit(d.transactOpts, slot)
	return err
}

func (d *RootChainService) CancelExits(slots []uint64) error {
	_, err := d.plasmaContract.CancelExits(d.transactOpts, slots)
	return err
}


func (d *RootChainService) FinalizeExit(slot uint64) error {
	_, err := d.plasmaContract.FinalizeExit(d.transactOpts, slot)
	return err
}

func (d *RootChainService) FinalizeExits(slots []uint64) error {
	_, err := d.plasmaContract.FinalizeExits(d.transactOpts, slots)
	return err
}

func (d *RootChainService) WithdrawBonds() error {
	_, err := d.plasmaContract.WithdrawBonds(d.transactOpts)
	return err
}

func (d *RootChainService) SubmitBlock(blockNum *big.Int, merkleRoot [32]byte) error {
	_, err := d.plasmaContract.SubmitBlock(d.transactOpts, blockNum, merkleRoot)
	return err
}

func (d *RootChainService) DebugCoinMetaData(slots []uint64) {
	if os.Getenv("DEBUG") != "true" {
		return
	}

	coins, err := d.plasmaContract.NumCoins(d.callOpts) //todo make this readonly
	fmt.Printf("Num coins -%v\n", coins)
	if err != nil {
		panic(err)
	}
	for _, y := range slots {
		//slot, c.depositBlock, c.denomination, c.owner, c.state
		returnSlot, _, _, owner, state, _, _, err := d.plasmaContract.GetPlasmaCoin(d.callOpts, y)
		fmt.Printf("Num coins -(slot)-%v -(returnSlot)-%v -(state)-%v -(owner)-%x\n", y, returnSlot, state, owner)

		if err != nil {
			panic(err)
		}
	}
}

func (d *RootChainService) ChallengedExitEventData(txHash common.Hash) (*plasma_cash.ChallengedExitEventData, error) {
	receipt, err := conn.TransactionReceipt(context.TODO(), txHash)
	if err != nil {
		return &plasma_cash.ChallengedExitEventData{}, err
	}
	if receipt == nil {
		return &plasma_cash.ChallengedExitEventData{}, errors.New("failed to retrieve tx receipt")
	}
	de, err := d.plasmaContract.ChallengedExitEventData(receipt)
	return &plasma_cash.ChallengedExitEventData{Slot: de.Slot, TxHash: de.TxHash, ChallengingBlockNumber: de.ChallengingBlockNumber}, err
}

func (d *RootChainService) DepositEventData(txHash common.Hash) (*plasma_cash.DepositEventData, error) {
	receipt, err := conn.TransactionReceipt(context.TODO(), txHash)
	if err != nil {
		return &plasma_cash.DepositEventData{}, err
	}
	if receipt == nil {
		return &plasma_cash.DepositEventData{}, errors.New("failed to retrieve tx receipt")
	}
	de, err := d.plasmaContract.DepositEventData(receipt)
	return &plasma_cash.DepositEventData{Slot: de.Slot, BlockNum: de.BlockNumber}, err
}

var conn *ethclient.Client

func InitClients(connStr string) {
	var err error
	conn, err = ethclient.Dial(connStr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
}

func NewRootChainService(callerName string, callerKey *ecdsa.PrivateKey, boundContract *ethcontract.RootChain) *RootChainService {
	auth := bind.NewKeyedTransactor(callerKey)
	// If gas price isn't set explicitely then go-ethereum will attempt to query the suggested gas
	// price, unfortunatley ganache-cli v6.1.2 seems to encode the gas price in a format go-ethereum
	// can't decode correctly, so this error is returned whenver you attempt to call a contract:
	// failed to suggest gas price: json: cannot unmarshal hex number with leading zero digits into Go value of type *hexutil.Big
	//
	// Earlier versions of ganache-cli don't seem to exhibit this issue, but they're broken in other
	// ways (logs aren't hex-encoded correctly).
	auth.GasPrice = big.NewInt(20000)
	auth.GasLimit = uint64(3141592)
	callerAddr := crypto.PubkeyToAddress(callerKey.PublicKey)
	return &RootChainService{
		Name:           callerName,
		callerKey:      callerKey,
		callerAddr:     callerAddr,
		plasmaContract: boundContract,
		transactOpts:   auth,
		callOpts: &bind.CallOpts{
			From: callerAddr,
		},
	}
}
