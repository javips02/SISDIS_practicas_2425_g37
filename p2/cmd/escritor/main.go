package main

import (
	"ra"
	"fmt"
	"math/rand"
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

//Every second there is a 10% chance that a new Read or Write operation is created
func taskCreator(ra *RASharedDB){

	string file = ""

	rand.Seed(time.Now().UnixNano())

	// Create a ticker that ticks every second
	ticker := time.NewTicker(1 * time.Second)

	// Infinite loop to keep checking every tick
	for range ticker.C {
		if rand.Intn(10) == 0 {
			if rand.Intn(2) == 0 {
				caracterCasual :=
				go writeOperation(ra)
			} else {
				go readOperation(ra)
			}
		}
	}
}

func writeOperation(ra *RASharedDB, s *string){
	string randomChar = randomChar()
	ra.PreProtocol()
	ra.FileMutex.Lock()
	fmt.Println("Adding %s to %s", randomChar, *s)
	ra.FileMutex += randomChar
	ra.FileMutex.Unlock()
	ra.PostProtocol()
}

func readOperation(){
	ra.PreProtocol()
	ra.FileMutex.Lock()
	fmt.Println("Read file: %s", r)
	ra.FileMutex.Lock()
	ra.PostProtocol()
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Println("Error: id missing: go run actor.go id")
		os.Exit(1)
	}
	ra = ra.New(args[1], "actors.txt")

	go taskCreator(ra)
}
