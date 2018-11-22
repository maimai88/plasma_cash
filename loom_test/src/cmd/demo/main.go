package main

import (
	"github.com/ningxin18/plasma_cash/loom_test/src/client"
	"context"
	"flag"
	"log"
	"math/big"
	"time"
)

func main() {

	maxIteration := 30
	sleepPerIteration := 2000 * time.Millisecond

	var hostile bool
	flag.BoolVar(&hostile, "hostile", false, "run the demo with a hostile Plasma Cash operator")
	flag.Parse()

	if hostile {
		log.Println("Testing with a hostile Plasma Cash operator")
	}

	client.InitClients("http://localhost:8545")
	client.InitTokenClient("http://localhost:8545")
	ganache, err := client.ConnectToGanache("http://localhost:8545")
	exitIfError(err)

	testCtx, err := client.SetupTest(hostile, "http://localhost:46658/query", "http://localhost:46658/rpc")
	exitIfError(err)

	alice := testCtx.Alice
	bob := testCtx.Bob
	charlie := testCtx.Charlie
	authority := testCtx.Authority

	slots := []uint64{}
	alice.DebugCoinMetaData(slots)

	// Give alice 5 tokens
	err = alice.TokenContract.Register()
	if err != nil {
		log.Fatalf("failed registering -%v\n", err)
	}

	aliceTokensStart, err := alice.TokenContract.BalanceOf()
	log.Printf("Alice has %d tokens\n", aliceTokensStart)

	if notEquals(aliceTokensStart, 5) {
		log.Fatalf("START: Alice has incorrect number of tokens")
	}
	bobTokensStart, err := bob.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Bob has %d tokens\n", bobTokensStart)
	if notEquals(bobTokensStart, 0) {
		log.Fatalf("START: Bob has incorrect number of tokens")
	}
	charlieTokensStart, err := charlie.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Charlie has %d tokens\n", charlieTokensStart)
	if notEquals(charlieTokensStart, 0) {
		log.Fatalf("START: Charlie has incorrect number of tokens")
	}

	currentBlock, err := authority.GetBlockNumber()
	exitIfError(err)

	_, err = ganache.HeaderByNumber(context.TODO(), nil)
	exitIfError(err)

	// Alice deposits 3 of her coins to the plasma contract and gets 3 plasma nft
	// utxos in return
	tokenID := big.NewInt(1)
	txHash := alice.Deposit(tokenID)
	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	deposit1, err := alice.RootChain.DepositEventData(txHash)
	exitIfError(err)
	slots = append(slots, deposit1.Slot)
	alice.DebugCoinMetaData(slots)

	txHash = alice.Deposit(tokenID.Add(tokenID, big.NewInt(1)))

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	deposit2, err := alice.RootChain.DepositEventData(txHash)
	exitIfError(err)
	slots = append(slots, deposit2.Slot)
	alice.DebugCoinMetaData(slots)

	txHash = alice.Deposit(tokenID.Add(tokenID, big.NewInt(2)))

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	deposit3, err := alice.RootChain.DepositEventData(txHash)
	exitIfError(err)
	slots = append(slots, deposit3.Slot)
	alice.DebugCoinMetaData(slots)

	//Alice to Bob, and Alice to Charlie. We care about the Alice to Bob
	// transaction

	account, err := charlie.TokenContract.Account()
	exitIfError(err)
	err = alice.SendTransaction(deposit2.Slot, deposit2.BlockNum, big.NewInt(1), account.Address) //randomTx
	exitIfError(err)

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	account, err = bob.TokenContract.Account()
	exitIfError(err)
	err = alice.SendTransaction(deposit3.Slot, deposit3.BlockNum, big.NewInt(1), account.Address) //aliceToBob
	exitIfError(err)

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	plasmaBlock1, err := authority.GetBlockNumber()

	// Bob to Charlie
	blkNum := plasmaBlock1
	account, err = charlie.TokenContract.Account() // the prev transaction was included in block 1000
	exitIfError(err)
	err = bob.SendTransaction(deposit3.Slot, blkNum, big.NewInt(1), account.Address) //bobToCharlie
	exitIfError(err)

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	// TODO: verify coin history

	//exitIfError(authority.SubmitBlock())
	plasmaBlock2, err := authority.GetBlockNumber()
	exitIfError(err)

	// Charlie should be able to submit an exit by referencing blocks 0 and 1 which
	// included his transaction.
	charlie.DebugCoinMetaData(slots)
	_, err = charlie.StartExit(deposit3.Slot, plasmaBlock1, plasmaBlock2)
	exitIfError(err)
	charlie.DebugCoinMetaData(slots)

	// After 8 days pass, charlie's exit should be finalizable
	_, err = ganache.IncreaseTime(context.TODO(), 8*24*3600)
	exitIfError(err)

	err = authority.FinalizeExit(deposit3.Slot)
	exitIfError(err)

	// Charlie should now be able to withdraw the utxo which included token 2 to his
	// wallet.

	charlie.DebugCoinMetaData(slots)
	err = charlie.Withdraw(deposit3.Slot)
	exitIfError(err)

	aliceTokensEnd, err := alice.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Alice has %d tokens\n", aliceTokensEnd)
	if notEquals(aliceTokensEnd, 2) {
		log.Fatal("END: Alice has incorrect number of tokens")
	}

	bobTokensEnd, err := bob.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Bob has %d tokens\n", bobTokensEnd)
	if notEquals(bobTokensEnd, 0) {
		log.Fatal("END: Bob has incorrect number of tokens")
	}
	charlieTokensEnd, err := charlie.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Charlie has %d  tokens\n", charlieTokensEnd)
	if notEquals(charlieTokensEnd, 1) {
		log.Fatal("END: Charlie has incorrect number of tokens")
	}

	log.Printf("Plasma Cash with ERC721 tokens success :)")

}

// not idiomatic go, but it cleans up this sample
func exitIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func notEquals(x *big.Int, y int64) bool {
	if x.Cmp(big.NewInt(y)) != 0 {
		return true
	} else {
		return false
	}
}
