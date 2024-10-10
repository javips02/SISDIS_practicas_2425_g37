/*
* AUTOR: Rafael Tolosana Calasanz
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2021
* FICHERO: ricart-agrawala.go
* DESCRIPCIÓN: Implementación del algoritmo de Ricart-Agrawala Generalizado en Go
 */
package ra

import (
	"fmt"
	"practica2/ms"
	"sync"
)

type Request struct {
	//Sender's clock
	Clock int
	//Sender's PID
	Pid int
}

type Reply struct {
	//Replier's clock
	Clock int
	//Replier's PID
	Pid int
	//The char we added to the file. If null, it was a read
	AddedChar string
}

type State string

// Define the enum values
const (
	Out    State = "out"
	Trying State = "trying"
	In     State = "in"
)

type RASharedDB struct {
	SendClock       int
	ReceiveClock    int
	OutRepCnt       int
	ReqCS           bool
	receiverReplies []bool
	ms              *ms.MessageSystem
	done            chan bool
	chrep           chan bool
	state           string
	mutex           sync.Mutex // mutex para proteger concurrencia sobre las variables
	File            string
	FileMutex       sync.Mutex
	repliesChannel  chan Reply
}

func New(me int, usersFile string) *RASharedDB {
	messageTypes := []ms.Message{Request, Reply}
	msgs := ms.New(me, usersFile, messageTypes)
	ra := RASharedDB{
		0,
		0,
		0,
		false,
		[]int{},
		&msgs,
		make(chan bool),
		make(chan bool),
		"out",
		&sync.Mutex{},
		"",
		"",
		&sync.Mutex{},
		make(chan Reply)}

	return &ra
}

// Pre: Verdad
// Post: Realiza  el  PreProtocol  para el  algoritmo de
//
//	Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol() {

	//Proceed
	//(Re)set internal variables
	ra.mutex.Lock()
	ra.SendClock = ra.ReceiveClock + 1
	request := Request{ra.SendClock, ra.ms.Me}
	ra.mutex.Unlock()

	//Send message to everyone and wait for the response
	ra.askForPermission(request)

	//if we reach here we're in critical section and simply return to caller

}

func (ra *RASharedDB) askForPermission(request Request) {
	ra.ms.SendAll(request)
	//TODO: Wait for all replies through ra.repliesChannel
}

// Pre: Verdad
// Post: Realiza  el  PostProtocol  para el  algoritmo de
//
//	Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol(addedChar string) {
	ra.mutex.Lock()
	ra.state = "out"
	reply := Reply{
		ra.SendClock,
		ra.ms.Me,
		addedChar}
	ra.mutex.Unlock()
	if addedChar != "" {
		//After a write, we send a reply to everyone so they can update their own file
		ra.ms.SendAll(reply)
	} else {
		//After a read, we send Replies only to deferred peers
	}
}

func (ra *RASharedDB) Stop() {
	ra.ms.Stop()
	ra.done <- true
}

func (ra *RASharedDB) messageReceiver() {
	for {
		msg := ra.ms.Receive()
		if req, ok := msg.(Request); ok {
			ra.mutex.Lock()
			//if it's a request, evaluate here if we can give ok or defer
			//If we're out of critical section, we have lower clock or matrix allows it
			if true {
				//send reply
			} else {
				//defer
			}
			ra.mutex.Unlock()
		} else if rep, ok := msg.(Reply); ok {
			ra.repliesChannel <- rep
		} else {
			fmt.Println("Unknown Message type")
		}
	}
}
