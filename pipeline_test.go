package ratchet_test

import (
	"fmt"
	"os"
	"strings"

	"github.com/dailyburn/ratchet/logger"
	"github.com/plandem/ratchet"
	"github.com/plandem/ratchet/data"
	"github.com/plandem/ratchet/processors"
)

func ExampleNewPipeline() {
	logger.LogLevel = logger.LevelSilent

	// A basic pipeline is created using one or more DataProcessor instances.
	hello := processors.NewIoReader(strings.NewReader("Hello world!"))
	stdout := processors.NewIoWriter(os.Stdout)
	pipeline := ratchet.NewPipeline(hello, stdout)

	err := <-pipeline.Run()

	if err != nil {
		fmt.Println("An error occurred in the ratchet pipeline:", err.Error())
	}

	// Output:
	// Hello world!
}

func ExampleNewBranchingPipeline() {
	logger.LogLevel = logger.LevelSilent

	// This example is very contrived, but we'll first create
	// DataProcessors that will spit out strings, do some basic
	// transformation, and then filter out all the ones that don't
	// match "HELLO".
	hello := processors.NewIoReader(strings.NewReader("Hello world"))
	hola := processors.NewIoReader(strings.NewReader("Hola mundo"))
	bonjour := processors.NewIoReader(strings.NewReader("Bonjour monde"))
	upperCaser := processors.NewFuncTransformer(func(d data.Payload) data.Payload {
		return data.Payload(strings.ToUpper(string(d)))
	})
	lowerCaser := processors.NewFuncTransformer(func(d data.Payload) data.Payload {
		return data.Payload(strings.ToLower(string(d)))
	})
	helloMatcher := processors.NewRegexpMatcher("HELLO")
	stdout := processors.NewIoWriter(os.Stdout)

	// Create the PipelineLayout that will run the DataProcessors
	layout, err := ratchet.NewPipelineLayout(
		// Stage 1 - spits out hello world in a few languages
		ratchet.NewPipelineStage(
			ratchet.Do(hello).Outputs(upperCaser, lowerCaser),
			ratchet.Do(hola).Outputs(upperCaser),
			ratchet.Do(bonjour).Outputs(lowerCaser),
		),
		// Stage 2 - transforms strings to upper and lower case
		ratchet.NewPipelineStage(
			ratchet.Do(upperCaser).Outputs(helloMatcher),
			ratchet.Do(lowerCaser).Outputs(helloMatcher),
		),
		// Stage 3 - only lets through strings that match "hello"
		ratchet.NewPipelineStage(
			ratchet.Do(helloMatcher).Outputs(stdout),
		),
		// Stage 4 - prints to STDOUT
		ratchet.NewPipelineStage(
			ratchet.Do(stdout),
		),
	)
	if err != nil {
		panic(err.Error())
	}

	// Create and run the Pipeline
	pipeline := ratchet.NewBranchingPipeline(layout)
	err = <-pipeline.Run()

	if err != nil {
		fmt.Println("An error occurred in the ratchet pipeline:", err.Error())
	}

	// Output:
	// HELLO WORLD
}
