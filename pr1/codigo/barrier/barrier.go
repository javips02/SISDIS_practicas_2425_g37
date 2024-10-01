package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Reads the file and saves the endpoints into an array
func readEndpoints(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close() //defer: will close the file once the function execution is over

	var endpoints []string
	scanner := bufio.NewScanner(file) //reads line by line
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" { //if line not empty save endpoint
			endpoints = append(endpoints, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return endpoints, nil
}

// Reads a msg from network conn, registers it inside a shared map (recieved) and uses a mutex to avoid concurrency
// problems if all msgs have been recieved except for one, send a signal to channel (barrierchan) indicating all nodes
// are ready to pass the barrier.
func handleConnection(conn net.Conn, barrierChan chan<- bool, received *map[string]bool, mu *sync.Mutex, n int) {
	defer conn.Close()        //close conn at the end of the function
	buf := make([]byte, 1024) //create buffer
	_, err := conn.Read(buf)  //reads from connection
	if err != nil {
		fmt.Println("Error reading from connection:", err)
		return
	}
	msg := string(buf)                                    //if not conn err,converts buffer to string (network to local)
	mu.Lock()                                             // gets lock for mutex "mu" to avoid race conditions
	(*received)[msg] = true                               //sends msg through recieved channel
	fmt.Println("Received ", len(*received), " elements") //prints info
	if len(*received) == n-1 {                            //check if I have received msgs from all other processes connected to me
		barrierChan <- true // tell barrier to "lift" so the processes can proceed
	}
	mu.Unlock() // free lock to let others access the critical section
}

// Looks in the file in arg[1], line arg[2] and returns the endpoint
// Endpoint: (IP adresse:port for each distributed process)
func getEndpoints() ([]string, int, error) {
	endpointsFile := os.Args[1]                 // get endpoint file name
	var endpoints []string                      // Declares array to save all the endpoints
	lineNumber, err := strconv.Atoi(os.Args[2]) //converts second argument into integer to be the line number
	if err != nil || lineNumber < 1 {
		fmt.Println("Invalid line number")
	} else if endpoints, err = readEndpoints(endpointsFile); err != nil {
		fmt.Println("Error reading endpoints:", err)
	} else if lineNumber > len(endpoints) {
		fmt.Printf("Line number %d out of range\n", lineNumber)
		err = errors.New("Line number out of range")
	}
	return endpoints, lineNumber, err
}

// Accept connections
// barrierChan: chan to indicate whether the conditions have been met on all nodes
func acceptAndHandleConnections(listener net.Listener, quitChannel chan bool,
	barrierChan chan bool, receivedMap *map[string]bool, mu *sync.Mutex, totalNodes int) {
	var n int // Connection counter initialisation
	for {     // infinite loop (until break ofc)
		select {
		case <-quitChannel: //if recieved signal in quitChannel just stop loop and exit
			fmt.Println("Recieved quitChannel signal, Stopping the listener...")
			close(quitChannel)
			listener.Close()
			break
		default:
			conn, err := listener.Accept() //accept new connection
			if err != nil {
				fmt.Println("Error accepting connection:", err)
				continue
			}
			n++ // Increment connection counter
			//go handleConnection(conn, barrierChan, receivedMap, mu, n) // goroutine to handle conn (new go thread)
			go handleConnection(conn, barrierChan, receivedMap, mu, totalNodes) // goroutine to handle conn (new go thread)
		}
	}
}

// send msg to all processes except for the one in line "lineNumber"
// Why? we exclude our process bc info is already on local
func notifyOtherDistributedProcesses(endPoints []string, lineNumber int) {
	for i, ep := range endPoints {
		if i+1 != lineNumber {
			go func(ep string) { // new go thread for each process to be notified
				for {
					conn, err := net.Dial("tcp", ep) // keep dialing with 1 second pauses until found
					if err != nil {
						fmt.Println("Error connecting to", ep, ":", err)
						time.Sleep(1 * time.Second)
						continue
					}
					_, err = conn.Write([]byte(strconv.Itoa(lineNumber))) // send my lineNumber to each endpoint
					if err != nil {                                       //try until sent
						fmt.Println("Error sending message:", err)
						conn.Close()
						continue
					}
					conn.Close()
					break
				}
			}(ep) //golang syntax to call the function with our current "ep" (as object)
		}
	}
}

func main() {
	var listener net.Listener //declare listener
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run main.go <endpoints_file> <line_number>")
	} else if endPoints, lineNumber, err := getEndpoints(); err != nil {
		fmt.Println("Error getting endpoints:", err)
	} else {
		// Get the endpoint for current process
		localEndpoint := endPoints[lineNumber-1]

		//Here we have in the host a list of endpoints as strings (same in all hosts or else won't work)
		if listener, err = net.Listen("tcp", localEndpoint); err != nil {
			fmt.Println("Error creating listener:", err)
		} else {
			fmt.Println("Listening on", localEndpoint)
			// Barrier synchronization
			var mu sync.Mutex
			// Write true in this channel to stop acceptAndHandleConn
			quitChannel := make(chan bool)       //Channel to indicate when to stop the listener
			receivedMap := make(map[string]bool) // channel to save all the recieved messages from thedifferent nodes
			barrierChan := make(chan bool)

			// Goroutine to handle connections from other processes
			go acceptAndHandleConnections(listener, quitChannel, barrierChan,
				&receivedMap, &mu, len(endPoints))

			// Notify other processes
			notifyOtherDistributedProcesses(endPoints, lineNumber)

			// Wait for all processes to reach the barrier, then close listener through handleCOnnections fun
			fmt.Println("Waiting for all the processes to reach the barrier")
			<-barrierChan
			fmt.Println("All processes reached the barrier, Sending quit signal to quitChannel")
			//close(quitChannel) //if you close channel program ends smoothly... if you try to send message through
			//it will get hung up and never get to the handleconnection to close listener __> CHECK LATER
			quitChannel <- true
		}
	}
}
