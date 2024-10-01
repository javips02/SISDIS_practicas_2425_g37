/*
* AUTOR: Rafael Tolosana Calasanz y Unai Arronategui
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2022
* FICHERO: server-draft.go
* DESCRIPCIÓN: contiene la funcionalidad esencial para realizar los servidores
*				correspondientes a la práctica 1
 */
package main

import (
	"encoding/gob"
	"log"
	"net"
	"os"
	"practica1/com"
	"runtime"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"


)

func runCommandOverSSH(ip, command string) error {
	// Define SSH config
	config := &ssh.ClientConfig{
		User: "a847803", // Replace with your SSH username
		auth := []ssh.AuthMethod{
			ssh.PublicKeys(yourPrivateKeySigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the SSH server
	client, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %v", err)
	}
	defer client.Close()

	// Create a new session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	// Execute the command
	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("command execution failed: %v, output: %s", err, output)
	}

	log.Printf("Command output: %s", output)
	return nil
}


func getEndpoints(file string) ([]string, error) {
    // Open the file workers.txt
    file, err := os.Open(file)
    if err != nil {
        return nil, fmt.Errorf("failed to open file: %v", err)
    }
    defer file.Close()

    // Create a slice to store endpoints
    var endpoints []string

    // Use bufio scanner to read the file line by line
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        // Append each line (an endpoint) to the slice
        endpoints = append(endpoints, scanner.Text())
    }

    // Check if there were any errors while reading the file
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("error reading file: %v", err)
    }

    return endpoints, nil
}

func startWorkers(endpoints []string, connectionsChan chan string) {
	for _, endpoint := range endpoints {
		// Split the endpoint into IP and port
		parts := strings.Split(endpoint, ":")
		if len(parts) != 2 {
			log.Printf("Invalid endpoint format: %s", endpoint)
			continue
		}
		
		ip := parts[0]
		port := parts[1]
		go worker(ip, port, connectionsChan)
	}
}

// Run worker via SSH
func worker (ip string, port string, connectionsChan chan net.Conn) {
	cmd := fmt.Sprintf("go run worker.go %s %s", ip, port)
	err := runCommandOverSSH(ip, cmd)
	if err != nil {
		log.Printf("Failed to start worker at %s:%s: %v", ip, port, err)
	} else {
		log.Printf("Worker started at %s:%s", ip, port)
	}
}

//Gets called every time a request is received
func requestsHandler(id int, connectionsChan chan net.Conn) {
	var request com.Request
	for {
		conn := <- connectionsChan
		decoder := gob.NewDecoder(conn)
		err := decoder.Decode(&request)
		com.CheckError(err)
		primes := findPrimes(request.Interval)
		reply := com.Reply{Id: request.Id, Primes: primes}
		encoder := gob.NewEncoder(conn)
		encoder.Encode(&reply)
		conn.Close()
	}
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Println("Error: endpoint missing: go run server.go ip:port")
		os.Exit(1)
	}
	endpoint := args[1]
	listener, err := net.Listen("tcp", endpoint)
	com.CheckError(err)

	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	
	//Contains the connection object created from clients' requests
	connectionsChan := make(chan net.Conn)

	if endPoints, err := getEndpoints("workers.txt"); err != nil {
		log.Println("Error: can't read workers.txt file")
		os.Exit(1)
	}

	startWorkers(endPoints, connectionsChan)

	log.Println("***** Listening for new connection in endpoint ", endpoint)
	//In here we wait for all clients requests and put them in connectionsChan
	//From there the goRoutines that send tasks to workers will handle them
	for {
		conn, err := listener.Accept()
		com.CheckError(err)
		connectionsChan <- conn

	}
}
