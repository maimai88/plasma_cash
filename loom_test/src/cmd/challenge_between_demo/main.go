package main

import (
	"github.com/ningxin18/plasma_cash/loom_test/src/client"

	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	authority := testCtx.Authority
	eve := testCtx.Eve

	aliceAccount, err := alice.TokenContract.Account()
	exitIfError(err)
	bobAccount, err := bob.TokenContract.Account()
	exitIfError(err)

	bobTokensStart, err := bob.TokenContract.BalanceOf()
	exitIfError(err)

	// Give Eve 5 tokens
	eve.Register()

	_, err = ganache.HeaderByNumber(context.TODO(), nil)
	exitIfError(err)

	// Eve deposits a coin
	txHash := eve.Deposit(big.NewInt(11))

	currentBlock, err := authority.GetBlockNumber()
	exitIfError(err)
	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}
	deposit1, err := eve.RootChain.DepositEventData(txHash)
	exitIfError(err)

	// Eve sends her plasma coin to Bob
	coin, err := eve.PlasmaCoin(deposit1.Slot)
	exitIfError(err)
	err = eve.SendTransaction(deposit1.Slot, coin.DepositBlockNum, big.NewInt(1), bobAccount.Address)
	exitIfError(err)

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	eveToBobBlockNum, err := authority.GetBlockNumber()
	exitIfError(err)

	// TODO: bob.WatchExits(deposit1.Slot)

	// Eve sends this same plasma coin to Alice
	err = eve.SendTransaction(deposit1.Slot, coin.DepositBlockNum, big.NewInt(1), aliceAccount.Address)
	exitIfError(err)

	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	eveToAliceBlock, err := authority.GetBlockNumber()
	exitIfError(err)

	fmt.Printf("Challenge Between - Alice attempts to exit slot %v prevBlock %v exitBlock %v\n",
		deposit1.Slot, coin.DepositBlockNum, eveToAliceBlock)
	// Alice attempts to exit here double-spent coin
	_, err = alice.StartExit(deposit1.Slot, coin.DepositBlockNum, eveToAliceBlock)
	exitIfError(err)

	fmt.Printf("Bob is challenging slot %v at block %v\n", deposit1.Slot, eveToBobBlockNum)
	// Alice's exit should be auto-challenged by Bob's client, but watching/auto-challenge hasn't
	// been implemented yet, so challenge the exit manually for now...
	_, err = bob.ChallengeBetween(deposit1.Slot, eveToBobBlockNum)
	exitIfError(err)

	fmt.Printf("Bob attempts to exit slot %v prevBlock %v exitBlock %v\n",
		deposit1.Slot, coin.DepositBlockNum, eveToBobBlockNum)
	_, err = bob.StartExit(deposit1.Slot, coin.DepositBlockNum, eveToBobBlockNum)
	exitIfError(err)

	// TODO: bob.StopWatchingExits(deposit1.Slot)

	_, err = ganache.IncreaseTime(context.TODO(), 8*24*3600)
	exitIfError(err)

	fmt.Println("Finalizing exits")
	exitIfError(authority.FinalizeExit(deposit1.Slot))

	fmt.Printf("Bob attempts to withdraw slot %v\n", deposit1.Slot)
	err = bob.Withdraw(deposit1.Slot)
	exitIfError(err)

	bobBalanceBefore, err := ganache.BalanceAt(context.TODO(), common.HexToAddress(bobAccount.Address), nil)
	exitIfError(err)
	fmt.Println("Bob attempts to withdraw bonds")
	err = bob.WithdrawBonds()
	time.Sleep(2 * time.Second)
	exitIfError(err)
	bobBalanceAfter, err := ganache.BalanceAt(context.TODO(), common.HexToAddress(bobAccount.Address), nil)
	exitIfError(err)

	if !(bobBalanceBefore.Cmp(bobBalanceAfter) < 0) {
		log.Fatal("END: Bob did not withdraw his bonds")
	}

	bobTokensEnd, err := bob.TokenContract.BalanceOf()
	exitIfError(err)

	fmt.Printf("Bob has %d tokens", bobTokensEnd)
	if bobTokensEnd.Cmp(bobTokensStart.Add(bobTokensStart, big.NewInt(1))) != 0 {
		log.Fatal("END: Bob has incorrect number of tokens")
	}

	fmt.Printf("Plasma Cash 'challengeBetween' success :)\n")

}

// not idiomatic go, but it cleans up this sample
func exitIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
