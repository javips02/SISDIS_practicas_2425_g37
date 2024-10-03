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
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"practica1/com"
	"strings"
	"time"
	"bufio"

	"golang.org/x/crypto/ssh"
)

func runCommandOverSSH(ip, command string) error {
	keyPath := "/home/conte/.ssh/id_ed25519" //pvt key

	// Read the private key file
	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	// Parse the private key to get a signer
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	// Define SSH config
	config := &ssh.ClientConfig{
		User: "conte", // Replace with your SSH username
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the SSH server
	client, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH: %v", err)
	}

	// Create a new session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}

	// Execute the command
	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("command execution failed: %v, output: %s", err, output)
	}
	log.Printf("Closing stuff")
	client.Close()
	session.Close()
	return nil
}

func getEndpoints(fileName string) ([]string, error) {
	// Open the file workers.txt
	file, err := os.Open(fileName)
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

func startWorkers(endpoints []string, connectionsChan chan net.Conn) {
	for _, endpoint := range endpoints {
		// Split the endpoint into IP and port
		parts := strings.Split(endpoint, ":")
		if len(parts) != 2 {
			log.Printf("Invalid endpoint format: %s", endpoint)
			continue
		}

		worker_ip := parts[0]
		worker_port := parts[1]
		go worker(worker_ip, worker_port, connectionsChan)
	}
}

// Start worker via SSH, listen for its connection and then
// send tasks to receive results
func worker(
	worker_ip string,
	worker_port string,
	connectionsChan chan net.Conn) {
	endpoint := worker_ip + ":" + worker_port
	cmd := fmt.Sprintf("cd /home/conte/Desktop/Lezioni/SSDD/SISDIS_practicas_2425_g37/pr1/codigo/cmd/servers/ && go run worker.go %s:%s", worker_ip, worker_port)
	log.Printf("Starting worker at %s:%s, if no error is printed ssh command is successful", worker_ip, worker_port)
	go runCommandOverSSH(worker_ip, cmd)

	time.Sleep(2 * time.Second)
	worker_conn, err := net.Dial("tcp", endpoint)
	com.CheckError(err)
	log.Printf("TCP ok at %s:%s", worker_ip, worker_port)
	worker_encoder := gob.NewEncoder(worker_conn)
	worker_decoder := gob.NewDecoder(worker_conn)

	for {
		var request com.Request
		conn := <- connectionsChan
		client_decoder := gob.NewDecoder(conn)
		err := client_decoder.Decode(&request)
		com.CheckError(err)

		worker_encoder.Encode(request)
		var reply com.Reply
		worker_decoder.Decode(&reply)

		client_encoder := gob.NewEncoder(conn)
		client_encoder.Encode(&reply)
		
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

	var endPoints []string
	if endPoints, err = getEndpoints("workers.txt"); err != nil {
		log.Println("Error: can't read workers.txt file")
		os.Exit(1)
	}

	go startWorkers(endPoints, connectionsChan)

	log.Println("***** Listening for new connection in endpoint ", endpoint)
	//In here we wait for all clients requests and put them in connectionsChan
	//From there the goRoutines that send tasks to workers will handle them
	for {
		conn, err := listener.Accept()
		com.CheckError(err)
		connectionsChan <- conn

	}
}
