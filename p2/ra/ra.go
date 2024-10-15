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
	receiverReplies []bool //revisar usos
	RepDefd         []bool
	ms              *ms.MessageSystem
	done            chan bool
	chrep           chan bool
	state           State
	mutex           sync.Mutex // mutex para proteger concurrencia sobre las variables
	File            string
	FileMutex       sync.Mutex
	repliesChannel  chan Reply
}

func New(me int, usersFile string, users []int) *RASharedDB {
	messageTypes := []ms.Message{Request{}, Reply{}}
	msgs := ms.New(me, usersFile, messageTypes)
	ra := RASharedDB{
		SendClock:      0,     // Inicializa reloj vectorial
		ReceiveClock:   0,     // Inicializa reloj de recepción
		OutRepCnt:      0,     // Inicializa OutRepCnt
		ReqCS:          false, // Inicializa ReqCS
		ms:             &msgs,
		RepDefd:        make([]bool, len(msgs.Peers)), // Inicializa con 'false' para todos los procesos
		done:           make(chan bool),
		chrep:          make(chan bool),
		state:          Out,          // Estado inicial
		File:           "",           // Inicializa File como string vacío
		FileMutex:      sync.Mutex{}, // Mutex no necesita ser referenciado
		repliesChannel: make(chan Reply),
	}
	return &ra
}

// Pre: Verdad
// Post: Realiza el PreProtocol para el algoritmo de Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol() {
	// (Re)set internal variables
	ra.mutex.Lock()

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
			ra.mutex.Unlock()
			break
		}
		ra.mutex.Unlock()

		// Esperamos en el canal hasta que llegue una respuesta
		<-ra.chrep
	}
}

// Pre: Verdad
// Post: Realiza  el  PostProtocol  para el  algoritmo de
//
//	Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol(addedChar string) {
	ra.mutex.Lock()
	ra.state = Out
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
		for i, val := range ra.RepDefd {
			if val {
				ra.ms.Send(i, reply)
			}
		}
	}
	// Now we clean deferred vector; all processes in it have been notified in this function
	for i := range ra.RepDefd {
		ra.RepDefd[i] = false
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
