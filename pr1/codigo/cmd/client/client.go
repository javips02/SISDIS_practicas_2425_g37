/*
* AUTOR: Rafael Tolosana Calasanz y Unai Arronategui
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2022
* FICHERO: client.go
* DESCRIPCIÓN: cliente completo para los cuatro escenarios de la práctica 1
 */
package main

import (
	"encoding/gob"
	"log"
	"net"
	"os"
	"practica1/com"
	"time"
	"fmt"
)

func sendEnd(endpoint string) {
	conn, err := net.Dial("tcp", endpoint)
	com.CheckError(err)

	encoder := gob.NewEncoder(conn)
	request := com.Request{Id: -1, Interval: com.TPInterval{Min: 0, Max: 0}}
	err = encoder.Encode(request)
	com.CheckError(err)

	conn.Close()
}

// sendRequest envía una petición (id, interval) al servidor. Una petición es un par id
// (el identificador único de la petición) e interval, el intervalo en el cual se desea que el servidor encuentre los
// números primos. La petición se serializa utilizando el encoder y una vez enviada la petición
// se almacena en una estructura de datos, junto con una estampilla
// temporal. Para evitar condiciones de carrera, la estructura de datos compartida se almacena en una Goroutine
// (handleRequests) y que controla los accesos a través de canales síncronos. En este caso, se añade una nueva
// petición a la estructura de datos mediante el canal requestTimeChan
func sendRequest(
	endpoint string,
	id int,
	interval com.TPInterval,
	requestTimeChan chan com.TimeCommEvent,
	replyTimeChan chan com.TimeCommEvent,
) {

	conn, err := net.Dial("tcp", endpoint)
	com.CheckError(err)
	encoder := gob.NewEncoder(conn)

	request := com.Request{Id: id, Interval: interval}
	timeRequest := com.TimeCommEvent{Id: id, T: time.Now()}
	err = encoder.Encode(request) // send request
	/* log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	log.Println("Client send Request with Id and Interval : ", request) */
	com.CheckError(err)

	requestTimeChan <- timeRequest
	go receiveReply(conn, replyTimeChan)
}

// handleRequests es una Goroutine que garantiza el acceso en exclusión mutua
// a la tabla de peticiones.
// La tabla de peticiones almacena todas las peticiones activas que se han
// realizado al servidor y cuándo se han realizado.
// El objetivo es que el cliente pueda calcular, para cada petición, cuál es el
// tiempo total desde que seenvía hasta que se recibe.
// Las peticiones le llegan a la goroutine a través del canal requestTimeChan.
// Por el canal replyTimeChan se indica que ha llegado una respuesta de una petición.
// En la respuesta, se obtiene también el timestamp de la recepción.
// Antes de eliminar una petición se imprime por la salida estándar el id de
// una petición y el tiempo transcurrido, observado por el cliente
// (tiempo de transmisión + tiempo de overheads + tiempo de ejecución efectivo)
func handleRequestsDelays(requestTimeChan chan com.TimeCommEvent, replyTimeChan chan com.TimeCommEvent) {
	//output.txt file for GNUPlot
	outFile, err := os.Create("output.txt")
	defer outFile.Close()
    if err != nil {
        fmt.Println("Error creating the file:", err)
        return
    }
	var printedRequests = 0
	var elapsedT time.Duration
	requestsTimes := make(map[int]time.Time)
	for {
		select {
		case timeRequest := <-requestTimeChan:
			requestsTimes[timeRequest.Id] = timeRequest.T
		case timereply := <-replyTimeChan:
			log.SetFlags(log.Lshortfile | log.Lmicroseconds)
			elapsedT = timereply.T.Sub(requestsTimes[timereply.Id])
			log.Println("-> Delay : ",
				elapsedT,
				", between request ", timereply.Id,
				" and its reply")
			fmt.Fprintln(outFile, printedRequests, " ", elapsedT)
			printedRequests++;
			delete(requestsTimes, timereply.Id)
		}
	}
}

// receiveReply recibe las respuestas (id, primos) del servidor.
//
//	Respuestas que corresponden con peticiones previamente realizadas.
//
// el encoder y una vez enviada la petición se almacena en una estructura de
// datos, junto con una estampilla temporal. Para evitar condiciones de carrera,
// la estructura de datos compartida se almacena en una Goroutine
// (handleRequests) y que controla los accesos a través de canales síncronos.
// En este caso, se añade una nueva petición a la estructura de datos mediante
// el canal requestTimeChan
func receiveReply(conn net.Conn, replyTimeChan chan com.TimeCommEvent) {
	var reply com.Reply
	decoder := gob.NewDecoder(conn)
	err := decoder.Decode(&reply) //  receive reply
	com.CheckError(err)
	timereply := com.TimeCommEvent{Id: reply.Id, T: time.Now()}

	/* log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	log.Println("Client receive reply for request Id  with resulting Primes =\n",
		reply) */

	replyTimeChan <- timereply

	conn.Close()
}

func main() {
	args := os.Args
	if len(args) != 2 {
		log.Println("Error: endpoint missing: go run client.go ip:port")
		os.Exit(1)
	}
	endpoint := args[1]
	requestTimeChan := make(chan com.TimeCommEvent)
	replyTimeChan := make(chan com.TimeCommEvent)
    
	go handleRequestsDelays(requestTimeChan, replyTimeChan)

	numIt := 10
	requestTmp := 6
	interval := com.TPInterval{Min: 1000, Max: 7000}
	tts := 1000 // time to sleep between consecutive requests

	for i := 0; i < numIt; i++ {
		for t := 1; t <= requestTmp; t++ {
			sendRequest(endpoint,
				i*requestTmp+t, interval, requestTimeChan, replyTimeChan)
		}
		time.Sleep(time.Duration(tts) * time.Millisecond)
	}

	sendEnd(endpoint) // send finish request
}
