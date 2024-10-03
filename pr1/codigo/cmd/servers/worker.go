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
)

// PRE: verdad = !foundDivisor
// POST: Returns true if n is prime
func isPrime(n int) (foundDivisor bool) {
	foundDivisor = false
	for i := 2; (i < n) && !foundDivisor; i++ {
		foundDivisor = (n%i == 0)
	}
	return !foundDivisor
}

// PRE: interval.A < interval.B
// POST: FindPrimes devuelve todos los números primos comprendidos en el
//
//	intervalo [interval.A, interval.B]
func findPrimes(interval com.TPInterval) (primes []int) {
	for i := interval.Min; i <= interval.Max; i++ {
		if isPrime(i) {
			primes = append(primes, i)
		}
	}
	return primes
}

//Gets called every time a request is received
func requestsHandler(
	id int, 
	requestChan chan com.Request,
	replyChan chan com.Reply) {
	
	for {
		request := <-requestChan
		primes := findPrimes(request.Interval)
		reply := com.Reply{Id: request.Id, Primes: primes}
		replyChan <- reply
	}
}

func sendRepliesToMaster(encoder *gob.Encoder, replyChan chan com.Reply) {
	for {
		reply := <- replyChan
		encoder.Encode(&reply)
	}
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Println("Error: endpoint missing: go run worker.go ip:port")
		os.Exit(1)
	}
	endpoint := args[1]
	listener, err := net.Listen("tcp", endpoint)
	com.CheckError(err)

	log.SetFlags(log.Lshortfile | log.Lmicroseconds)

	// Get the number of logical CPUs (threads)
	//We'll use this number to launch a pool of goroutines
    phisicalThreads := runtime.NumCPU()
	requestChan := make(chan com.Request)
	replyChan := make(chan com.Reply)

	var encoder *gob.Encoder
	var decoder *gob.Decoder
	phisicalThreads = 1
	for i := 0; i<phisicalThreads; i++ {
		go requestsHandler(i, requestChan, replyChan)
	}
	

	log.Println("***** Listening for new connection in endpoint ", endpoint)
	conn, err := listener.Accept()
	decoder = gob.NewDecoder(conn)
	encoder = gob.NewEncoder(conn)
	go sendRepliesToMaster(encoder, replyChan)
	com.CheckError(err)
	for {
		var request com.Request
		err := decoder.Decode(&request)
		com.CheckError(err)
		requestChan <- request
	}
}
