package main

import (
	"flag"
	"github.com/rkoesters/trumpet"
	"github.com/rkoesters/trumpet/generator/count"
	"github.com/rkoesters/trumpet/generator/dummy"
	"github.com/rkoesters/trumpet/generator/markov"
	"github.com/rkoesters/trumpet/generator/multi"
	"github.com/rkoesters/trumpet/generator/verbatim"
	"github.com/rkoesters/trumpet/scheduler/sametime"
	"github.com/rkoesters/trumpet/scheduler/timer"
	"github.com/rkoesters/trumpet/source/twitter"
	"log"
	"math/rand"
	"os"
	"time"
)

var (
	generator = flag.String("generator", "markov", "name of the generator to use")
	scheduler = flag.String("scheduler", "sametime", "name of the scheduler to use")

	markovLength = flag.Int("markov-length", 3, "length of each prefix for the markov generator")
	timerFreq    = flag.Duration("timer-freq", time.Minute, "frequency for the timer scheduler")
)

func main() {
	flag.Parse()

	// We don't take any arguments, only flags.
	if flag.NArg() != 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Seed the random number generator.
	rand.Seed(time.Now().Unix())

	// Use multi.Generator to multiplex our training data over multiple
	// trumpet.Generators.
	m := multi.New()

	// Use count.Generator to keep track of our input size.
	counter := count.New()
	m.AddTrainer(counter)

	// Use verbatim.Generator to make sure we don't copy a tweet
	// verbatim.
	duplicateChecker := verbatim.New()
	m.AddTrainer(duplicateChecker)

	// Pick our generator.
	var gen trumpet.Generator
	switch *generator {
	case "dummy":
		gen = &dummy.Generator{}
	case "markov":
		gen = markov.NewChain(*markovLength)
	default:
		log.Fatalf("unknown generator: %v", *generator)
	}
	m.AddTrainer(gen)
	m.SetGenerator(gen)

	// Pick our scheduler.
	var sched trumpet.Scheduler
	switch *scheduler {
	case "timer":
		sched = timer.New(*timerFreq)
	case "sametime":
		sched = sametime.New()
	default:
		log.Fatalf("unknown scheduler: %v", *scheduler)
	}

	// Prepare the twitter layer.
	twitter.Init()

	// Get list of user IDs to learn from.
	userIDs, err := twitter.GetFriends()
	if err != nil {
		log.Fatal(err)
	}

	// Start fetching our input.
	incomingTweets := make(chan string)
	for _, userID := range userIDs {
		go twitter.GetPastTweets(userID, incomingTweets)
	}
	go twitter.ListenForTweets(userIDs, incomingTweets, sched)

	// Start writing tweets according to our scheduler.
	outgoingTweets := composeTweets(m, sched, duplicateChecker)

	// Main loop.
	for {
		select {
		case t := <-incomingTweets:
			log.Printf("IN(((%v)))", t)
			m.Train(t)
			log.Printf("input size: %v", *counter)
		case t := <-outgoingTweets:
			log.Printf("OUT(((%v)))", t)
			twitter.Tweet(t)
		}
	}
}

func composeTweets(gen trumpet.Generator, sched trumpet.Scheduler, checker *verbatim.Generator) <-chan string {
	c := make(chan string)
	go func() {
		for {
			<-sched.Chan()

			for {
				t := gen.Generate(280)
				if !checker.Exists(t) {
					c <- t
					break
				}
			}
		}
	}()
	return c
}
