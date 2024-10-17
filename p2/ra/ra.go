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
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"practica2/ms"
	"sync"
	"time"
)

type Request struct {
	//Sender's PID
	Pid int
	//If it's a read or a write
	OpType OpType
	//Vectorial clock
	VectorialClock []int
}

type Reply struct {
	//Replier's PID
	Pid int
	//The char we added to the file. If null, it was a read
	AddedChar string
	//Vectorial clock
	VectorialClock []int
}

type State string
type OpType string

// Define the enum values
const (
	Out    State = "out"
	Trying State = "trying"
	In     State = "in"
)

const (
	Read  OpType = "read"
	Write OpType = "write"
)

type RASharedDB struct {
	deferredReplies []bool
	vectorialClock  []int
	ms              *ms.MessageSystem
	done            chan bool
	state           State
	opType          OpType
	mutex           sync.Mutex // mutex para proteger concurrencia sobre las variables
	File            string
	FileMutex       sync.Mutex
	repliesChannel  chan Reply
}

func New(me int, usersFile string) *RASharedDB {
	messageTypes := []ms.Message{Request{}, Reply{}}
	msgs := ms.New(me, usersFile, messageTypes)
	ra := RASharedDB{
		vectorialClock:  make([]int, len(msgs.Peers)),
		ms:              &msgs,
		deferredReplies: make([]bool, len(msgs.Peers)), // Inicializa con 'false' para todos los procesos
		done:            make(chan bool),
		state:           Out,            // Estado inicial
		File:            "myFileBlaBla", // Inicializa File como string vacío
		FileMutex:       sync.Mutex{},   // Mutex no necesita ser referenciado
		repliesChannel:  make(chan Reply, 2*len(msgs.Peers)),
	}
	for i := 0; i < len(msgs.Peers); i++ {
		ra.vectorialClock[i] = 0
		ra.deferredReplies[i] = false
	}
	go ra.messageReceiver()
	return &ra
}

// Pre: Verdad
// opType puede ser Read o Write
// Post: Realiza el PreProtocol para el algoritmo de Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol(opType OpType) {

	// (Re)set internal variables
	ra.mutex.Lock()
	ra.state = Trying
	ra.opType = opType
	// Incrementar el reloj lógico
	ra.vectorialClock[ra.ms.Me]++

	// Crear el mensaje de petición
	request := Request{
		VectorialClock: ra.vectorialClock,
		Pid:            ra.ms.Me,
		OpType:         opType,
	}

	ra.mutex.Unlock()

	// Enviar mensaje de petición a todos los procesos y esperar respuestas
	ra.askForPermission(request)

	// Al llegar aquí, se tiene acceso a la sección crítica

}

// Envía mensaje de solicitud de entrada a la SC a todos los nodos y espera
// respuesta de todos ellos antes de entrar
func (ra *RASharedDB) askForPermission(request Request) {

	ra.ms.SendAll(request)
	//time.Sleep(2 * time.Second)
	receivedReplies := 0
	// Esperar respuestas de todos los procesos
	for {

		// Esperamos en el canal hasta que llegue una respuesta
		reply := <-ra.repliesChannel
		receivedReplies++

		if receivedReplies == len(ra.ms.Peers)-1 { //aprovechamos el vector de peers que nos dice cuántos compañeros hay en el sisdis
			// Si hemos recibido respuestas de todos (menos nosotros mismos), salimos del bucle
			fmt.Printf("GOT ALL REPLIES, entering CS\n")
			break
		} else {
			fmt.Printf("Got reply %d of %d by peer %d\n", receivedReplies, len(ra.ms.Peers)-1, reply.Pid)
		}

	}
	ra.mutex.Lock()
	ra.state = In
	ra.mutex.Unlock()
}

// Pre: Verdad
// Post: Realiza  el  PostProtocol  para el  algoritmo de
//
//	Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol(addedChar string) {
	ra.mutex.Lock()

	ra.state = Out
	reply := Reply{
		Pid:            ra.ms.Me,
		AddedChar:      addedChar,
		VectorialClock: ra.vectorialClock,
	}

	ra.mutex.Unlock()
	if addedChar != "" {
		//After a write, we send a reply to everyone so they can update their own file
		ra.ms.SendAll(reply)
		fmt.Printf("Sending update to all after write CS")
	} else {
		//After a read, we send Replies only to deferred peers
		for i, val := range ra.deferredReplies {
			if val {
				ra.ms.Send(i, reply)
				fmt.Printf("Sending deferred reply to %d", i)
			}
		}
	}
	// Now we clean deferred vector; all processes in it have been notified by this function
	for i := range ra.deferredReplies {
		ra.deferredReplies[i] = false
	}
	log.Println(ra.vectorialClock)
}

func (ra *RASharedDB) Stop() {
	ra.ms.Stop()
	ra.done <- true
}

func (ra *RASharedDB) messageReceiver() {
	for {
		msg := ra.ms.Receive()
		if req, ok := msg.(Request); ok {
			//In this case we also update our internal clock
			ra.vectorialClock[ra.ms.Me] = req.VectorialClock[req.Pid]
			ra.updateVectorialClock(req.VectorialClock)
			ra.mutex.Lock()
			ourClock := ra.vectorialClock[ra.ms.Me]
			theirClock := req.VectorialClock[req.Pid]
			//If we're out of CS and not trying
			canGivePermission1 := ra.state == Out
			if canGivePermission1 {
				fmt.Printf("Condition 1 met\n")
			}
			//Or if we both want to read
			canGivePermission2 := (ra.opType == Read && req.OpType == Read)
			if canGivePermission2 {
				fmt.Printf("Condition 2 met\n")
			}
			//Or if the sender's sendClock is lower than ours AND we're not already in CS
			canGivePermission3 := (ourClock > theirClock && ra.state != In)
			if canGivePermission3 {
				fmt.Printf("Condition 3 met\n")
			}

			canGivePermission := canGivePermission1 || canGivePermission2 || canGivePermission3

			ra.mutex.Unlock()

			//if it's a request, evaluate here if we can give ok or defer
			//If we're out of critical section, we have lower clock or matrix allows it
			if canGivePermission {
				//This is not after a write, so we pass "" as second
				if ra.ms.Me == 2 {
					fmt.Printf("Sleeping 5 secs\n")
					time.Sleep(5 * time.Second)
				}
				ra.sendPermission(req.Pid, "")
				fmt.Printf("Got CS request from %d, giving permission\n", req.Pid)
			} else {

				ra.deferredReplies[req.Pid] = true

				fmt.Printf("Got CS request from %d, deferring\n", req.Pid)
			}
		} else if rep, ok := msg.(Reply); ok {
			//We update our vectorial clock but don't update *our* clock
			//because it's a received reply event

			ra.updateVectorialClock(rep.VectorialClock)
			if rep.AddedChar != "" {
				fmt.Printf("Got non empty reply, updating file\n")
				ra.File = ra.File + rep.AddedChar
			}

			//If we were not even trying to enter, then it's an after write update
			//and we don't need to put
			ra.mutex.Lock()
			if ra.state != Out {
				ra.repliesChannel <- rep
			}
			ra.mutex.Unlock()

		}
	}
}

// Sends a Reply to node number pid, added char should be not null only
// after exiting a WRITE CRITICAL SECTION, empty string ("") otherwise
func (ra *RASharedDB) sendPermission(pid int, addedChar string) {
	ra.mutex.Lock()
	reply := Reply{
		Pid:            ra.ms.Me,
		AddedChar:      addedChar,
		VectorialClock: ra.vectorialClock,
	}
	ra.mutex.Unlock()
	ra.ms.Send(pid, reply)
}

// Updates our vectorial clock by using one received in a message.
func (ra *RASharedDB) updateVectorialClock(receivedClock []int) {
	ra.mutex.Lock()
	for i := 0; i < len(ra.vectorialClock); i++ {
		if receivedClock[i] > ra.vectorialClock[i] {
			ra.vectorialClock[i] = receivedClock[i]
		}
	}
	ra.mutex.Unlock()

}

// Convert Request to []byte using Gob
func (r Request) ToBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(r)
	if err != nil {
		log.Fatalf("Failed to encode Request: %v", err)
	}
	return buf.Bytes()
}

// Convert Reply to []byte using Gob
func (r Reply) ToBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(r)
	if err != nil {
		log.Fatalf("Failed to encode Reply: %v", err)
	}
	return buf.Bytes()
}
