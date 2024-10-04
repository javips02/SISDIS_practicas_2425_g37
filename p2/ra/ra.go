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
    "ms"
    "sync"
)

type Request struct{
    Clock   int
    Pid     int
}

type Reply struct{
    CharAdded   string
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
    OutRepCnt   int
    ReqCS       boolean
    RepDefd     bool[]
    ms          *MessageSystem
    done        chan bool
    chrep       chan bool
    state       string
    Mutex       sync.Mutex // mutex para proteger concurrencia sobre las variables
    File        string
    FileMutex   sync.Mutex
}



func New(me int, usersFile string) (*RASharedDB) {
    messageTypes := []Message{Request, Reply}
    msgs = ms.New(me, usersFile string, messageTypes)
    ra := RASharedDB{0,
        0,
        0,
        false, 
        []int{}, 
        &msgs,  
        make(chan bool),  
        make(chan bool), 
        &sync.Mutex{}, 
        "", 
        &sync.Mutex{}}
    
    return &ra
}

//Pre: Verdad
//Post: Realiza  el  PreProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PreProtocol(){
    ra.Mutex.Lock()
    ra.SendClock = ra.ReceiveClock + 1
    sendClock := ra.SendClock
    request := Request{ra.SendClock, ra.ms.Me}
    ra.Mutex.Unlock()
    ms.SendAll(request)
}

//Pre: Verdad
//Post: Realiza  el  PostProtocol  para el  algoritmo de
//      Ricart-Agrawala Generalizado
func (ra *RASharedDB) PostProtocol(){
    // TODO completar
}

func (ra *RASharedDB) Stop(){
    ra.ms.Stop()
    ra.done <- true
}

// Returns our own endpoint
func (ra *RASharedDB) GetCurrentEndpoint(string) {
	return ra.ms.GetCurrentEndpoint()
}
