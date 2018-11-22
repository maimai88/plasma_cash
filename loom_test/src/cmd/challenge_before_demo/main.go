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

	dan := testCtx.Dan
	trudy := testCtx.Trudy
	mallory := testCtx.Mallory
	authority := testCtx.Authority

	danAccount, err := dan.TokenContract.Account()
	exitIfError(err)
	trudyAccount, err := trudy.TokenContract.Account()
	exitIfError(err)
	malloryAccount, err := mallory.TokenContract.Account()
	exitIfError(err)

	curBlockNum, err := authority.GetBlockNumber()
	fmt.Printf("Current Plasma Block %v\n", curBlockNum)

	// Give Dan 5 tokens
	dan.TokenContract.Register()

	_, err = ganache.HeaderByNumber(context.TODO(), nil)
	exitIfError(err)

	// Dan deposits a coin
	currentBlock, err := authority.GetBlockNumber()
	exitIfError(err)
	txHash := dan.Deposit(big.NewInt(16))
	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}

	depEvent, err := mallory.RootChain.DepositEventData(txHash)
	exitIfError(err)
	depositSlot1 := depEvent.Slot

	danTokenStart, err := dan.TokenContract.BalanceOf()
	exitIfError(err)

	coin, err := dan.RootChain.PlasmaCoin(depositSlot1)
	exitIfError(err)

	// TODO: Dan should start watching for exits of depositSlot1

	// Trudy sends her invalid coin (which she doesn't own) to Mallory
	exitIfError(trudy.SendTransaction(depositSlot1, coin.DepositBlockNum, big.NewInt(1), malloryAccount.Address))
	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}
	trudyToMalloryBlockNum, err := authority.GetBlockNumber()
	exitIfError(err)

	// Mallory sends the invalid coin back to Trudy
	exitIfError(mallory.SendTransaction(depositSlot1, trudyToMalloryBlockNum, big.NewInt(1), trudyAccount.Address))
	currentBlock, err = client.PollForBlockChange(authority, currentBlock, maxIteration, sleepPerIteration)
	if err != nil {
		panic(err)
	}
	malloryToTrudyBlockNum, err := authority.GetBlockNumber()
	exitIfError(err)

	fmt.Println("Trudy attempts to exit...")
	// Trudy attempts to exit her invalid coin
	_, err = trudy.StartExit(depositSlot1, trudyToMalloryBlockNum, malloryToTrudyBlockNum)
	exitIfError(err)

	fmt.Println("Dan attempts to challenge...")
	// Dan challenges Trudy's exit (in practice this will be done automatically by Dan's client
	// (once watching is implemented)
	_, err = dan.ChallengeBefore(depositSlot1, big.NewInt(0), coin.DepositBlockNum)
	exitIfError(err)

	// Let 8 days pass without any response to the challenge
	_, err = ganache.IncreaseTime(context.TODO(), 8*24*3600)
	exitIfError(err)

	fmt.Println("Finalizing exits...")
	exitIfError(authority.FinalizeExit(depositSlot1))

	fmt.Println("Dan attempts to exit...")
	// Having successfully challenged Trudy's exit Dan should be able to exit the coin
	_, err = dan.StartExit(depositSlot1, big.NewInt(0), coin.DepositBlockNum)
	exitIfError(err)

	// TODO: Dan should stop watching exits of depositSlot1

	// Jump forward in time by another 8 days
	_, err = ganache.IncreaseTime(context.TODO(), 8*24*3600)
	exitIfError(err)

	exitIfError(authority.FinalizeExit(depositSlot1))

	exitIfError(dan.Withdraw(depositSlot1))

	danBalanceBefore, err := ganache.BalanceAt(context.TODO(), common.HexToAddress(danAccount.Address), nil)
	exitIfError(err)
	exitIfError(dan.WithdrawBonds())
	time.Sleep(2 * time.Second)
	danBalanceAfter, err := ganache.BalanceAt(context.TODO(), common.HexToAddress(danAccount.Address), nil)
	exitIfError(err)

	if !(danBalanceBefore.Cmp(danBalanceAfter) < 0) {
		log.Fatal("END: Dan did not withdraw his bonds")
	}

	danTokensEnd, err := dan.TokenContract.BalanceOf()
	exitIfError(err)
	log.Printf("Dan has %v tokens", danTokensEnd)
	if danTokensEnd.Cmp(danTokenStart.Add(danTokenStart, big.NewInt(1))) != 0 {
		log.Fatal("END: Dan has incorrect number of tokens")
	}

	log.Printf("Plasma Cash `challengeBefore` success :)")
}

// not idiomatic go, but it cleans up this sample
func exitIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
