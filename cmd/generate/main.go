package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/solresol/ultrametric-trees/internal/processor"
)

func main() {
	// Define command-line flags
	inputDB := flag.String("input", "", "Path to input SQLite database")
	traverseDB := flag.String("traverse", "", "Path to traverse_synset SQLite database")
	outputDB := flag.String("output", "", "Path to output SQLite database")

	// Parse command-line flags
	flag.Parse()

	// Check if all required flags are provided
	if *inputDB == "" || *traverseDB == "" || *outputDB == "" {
		fmt.Println("Please provide all required database paths")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create processor
	proc, err := processor.NewProcessor(*inputDB, *traverseDB, *outputDB)
	if err != nil {
		log.Fatalf("Error creating processor: %v", err)
	}
	defer proc.Close()

	// Process data
	err = proc.Process()
	if err != nil {
		log.Fatalf("Error processing data: %v", err)
	}

	fmt.Println("Training data generation completed")
}
