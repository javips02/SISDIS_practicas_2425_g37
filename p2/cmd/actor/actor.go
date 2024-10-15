package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"practica2/com"
	ram "practica2/ra"
	"strconv"
	"time"
)

func randomChar() string {
	// ASCII ranges for:
	// '0'-'9' -> 48-57
	// 'A'-'Z' -> 65-90
	// 'a'-'z' -> 97-122
	ranges := []struct{ min, max int }{
		{48, 57},  // Numbers '0'-'9'
		{65, 90},  // Uppercase letters 'A'-'Z'
		{97, 122}, // Lowercase letters 'a'-'z'
	}

	// Pick a random range
	selectedRange := ranges[rand.Intn(len(ranges))]

	// Generate a random character within the selected range
	return string(byte(rand.Intn(selectedRange.max-selectedRange.min+1) + selectedRange.min))
}

// Every second there is a 10% chance that a new Read or Write operation is created
func taskCreator(ra *ram.RASharedDB) {
	return
	rand.Seed(0)

	// Create a ticker that ticks every second
	ticker := time.NewTicker(1 * time.Second)

	// Infinite loop to keep checking every tick
	for range ticker.C {
		if rand.Intn(10) == 0 {
			if rand.Intn(2) == 0 {
				writeOperation(ra)
			} else {
				readOperation(ra)
			}
		}
	}
}

func writeOperation(ra *ram.RASharedDB) {
	randomChar := randomChar()
	fmt.Printf("Asking for write consensus")
	ra.PreProtocol(ram.Write)
	fmt.Printf("Got consensus. Adding %s to %s\n", randomChar, ra.File)
	time.Sleep(2 * time.Second)
	ra.File += randomChar
	ra.PostProtocol(randomChar)
	fmt.Printf("**PostProtocol completed**\n")
}

func readOperation(ra *ram.RASharedDB) {
	fmt.Printf("Asking for read consensus\n")
	ra.PreProtocol(ram.Read)
	ra.FileMutex.Lock()
	fmt.Printf("File is: %s\n", ra.File)
	time.Sleep(2 * time.Second)
	ra.FileMutex.Unlock()
	ra.PostProtocol("")
	fmt.Printf("**PostProtocol completed**\n")
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Println("Error: id missing: go run actor.go id")
		os.Exit(1)
	}
	pid, err := strconv.Atoi(args[1])
	com.CheckError(err)
	ra := ram.New(pid, "actors.txt")

	go taskCreator(ra)

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n\nType 'r' or 'w':")
	for {
		input, _ := reader.ReadByte()
		switch input {
		case 'w':
			writeOperation(ra)
			fmt.Printf("\n\nType 'r' or 'w':")
		case 'r':
			readOperation(ra)
			fmt.Printf("\n\nType 'r' or 'w':")
		}
	}
}
