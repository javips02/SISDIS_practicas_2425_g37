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
	//Contains 
	freeWorkersChan := make(chan net.Conn)

	log.Println("***** Listening for new connection in endpoint ", endpoint)
	for {
		conn, err := listener.Accept()
		com.CheckError(err)
		connectionsChan <- conn

	}
}
