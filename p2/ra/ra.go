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
	//If it's a read or a write
	OpType OpType
}

type Reply struct {
	//Replier's PID
	Pid int
	//The char we added to the file. If null, it was a read
	AddedChar string
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
	SendClock       int
	ReceiveClock    int
	OutRepCnt       int
	ReqCS           bool
	deferredReplies []bool
	ms              *ms.MessageSystem
	done            chan bool
	chrep           chan bool
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
		SendClock:       0,     // Inicializa reloj vectorial
		ReceiveClock:    0,     // Inicializa reloj de recepción
		OutRepCnt:       0,     // Inicializa OutRepCnt
		ReqCS:           false, // Inicializa ReqCS
		ms:              &msgs,
		deferredReplies: make([]bool, len(msgs.Peers)), // Inicializa con 'false' para todos los procesos
		done:            make(chan bool),
		chrep:           make(chan bool),
		state:           Out,          // Estado inicial
		File:            "",           // Inicializa File como string vacío
		FileMutex:       sync.Mutex{}, // Mutex no necesita ser referenciado
		repliesChannel:  make(chan Reply),
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
	ra.SendClock = ra.ReceiveClock + 1

	// Crear el mensaje de petición
	request := Request{
		Clock: ra.SendClock,
		Pid:   ra.ms.Me,
	}
	ra.ReqCS = true
	ra.OutRepCnt = 0 // Reiniciar el contador de respuestas pendientes

	ra.mutex.Unlock()

	// Enviar mensaje de petición a todos los procesos y esperar respuestas
	ra.askForPermission(request)

	// Al llegar aquí, se tiene acceso a la sección crítica

}

// Envía mensaje de solicitud de entrada a la SC a todos los nodos y espera
// respuesta de todos ellos antes de entrar
func (ra *RASharedDB) askForPermission(request Request) {
	ra.ms.SendAll(request)

	// Esperar respuestas de todos los procesos
	for {
		ra.mutex.Lock()
		if ra.OutRepCnt == len(ra.ms.Peers)-1 { //aprovechamos el vector de peers que nos dice cuántos compañeros hay en el sisdis
			// Si hemos recibido respuestas de todos (menos nosotros mismos), salimos del bucle
			break
		}
		ra.mutex.Unlock()

		// Esperamos en el canal hasta que llegue una respuesta
		<-ra.chrep
	}
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
		Pid:       ra.ms.Me,
		AddedChar: addedChar,
	}

	ra.mutex.Unlock()
	if addedChar != "" {
		//After a write, we send a reply to everyone so they can update their own file
		ra.ms.SendAll(reply)
	} else {
		//After a read, we send Replies only to deferred peers
		for i, val := range ra.deferredReplies {
			if val {
				ra.ms.Send(i, reply)
			}
		}
	}
	// Now we clean deferred vector; all processes in it have been notified by this function
	for i := range ra.deferredReplies {
		ra.deferredReplies[i] = false
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
			//If we're out of CS and not trying
			canGivePermission := ra.state == Out
			//Or if we both want to read
			canGivePermission = canGivePermission || (ra.opType == Read && req.OpType == Read)
			//Or if the sender's sendClock is lower than ours AND we're not already in CS
			canGivePermission = canGivePermission || (req.Clock < ra.SendClock && ra.state != In)
			ra.mutex.Unlock()

			//if it's a request, evaluate here if we can give ok or defer
			//If we're out of critical section, we have lower clock or matrix allows it
			if canGivePermission {
				//This is not after a write, so we pass "" as second
				ra.sendPermission(req.Pid, "")
			} else {
				ra.deferredReplies[req.Pid] = true
			}
			ra.mutex.Unlock()
		} else if rep, ok := msg.(Reply); ok {
			ra.repliesChannel <- rep
		} else {
			fmt.Println("Unknown Message type")
		}
	}
}

// Sends a Reply to node number pid, added char should be not null only
// after exiting a WRITE CRITICAL SECTION, empty string ("") otherwise
func (ra *RASharedDB) sendPermission(pid int, addedChar string) {
	reply := Reply{
		Pid:       ra.ms.Me,
		AddedChar: addedChar,
	}
	ra.ms.Send(pid, reply)
}
