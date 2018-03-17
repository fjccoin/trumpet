package main

import (
	"flag"
	"github.com/rkoesters/trumpet"
	"github.com/rkoesters/trumpet/generator/markov"
	"github.com/rkoesters/trumpet/generator/multi"
	"github.com/rkoesters/trumpet/source/twitter"
	"log"
	"math/rand"
	"os"
	"time"
)

var (
	freq = flag.Duration("freq", time.Hour, "time between tweets")
)

func main() {
	flag.Parse()

	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}

	rand.Seed(time.Now().Unix())

	twitter.Init()

	userIDs, err := twitter.GetFriends()
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan string)

	gen := multi.New()
	brain := markov.NewChain(2)
	gen.AddTrainer(brain)
	gen.SetGenerator(brain)

	for _, userID := range userIDs {
		go twitter.GetPastTweets(userID, c)
	}
	go twitter.ListenForTweets(userIDs, c)

	outgoingTweets := composeTweets(gen)

	for {
		select {
		case t := <-c:
			log.Printf("IN(((%v)))", t)
			gen.Train(t)
		case t := <-outgoingTweets:
			log.Printf("OUT(((%v)))", t)
			twitter.Tweet(t)
		}
	}
}

func composeTweets(gen trumpet.Generator) <-chan string {
	c := make(chan string)
	go func() {
		for {
			time.Sleep(*freq)

			c <- gen.Generate(280)
		}
	}()
	return c
}
