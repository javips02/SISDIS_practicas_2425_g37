/*
* AUTOR: Rafael Tolosana Calasanz
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2021
* FICHERO: ms.go
* DESCRIPCIÓN: Implementación de un sistema de mensajería asíncrono, insipirado en el Modelo Actor
 */
package ms

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"practica2/com"

	"github.com/DistributedClocks/GoVector/govec"
)

type Message interface {
	ToBytes() []byte
}

type MessageSystem struct {
	mbox  chan Message
	Peers []string
	done  chan bool
	Me    int

	logger  *govec.GoLog
	logOpts govec.GoLogOptions
}

const (
	MAXMESSAGES = 10000
)

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func parsePeers(path string) (lines []string) {
	file, err := os.Open(path)
	checkError(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

// Pre: pid en {1..n}, el conjunto de procesos del SD
// Post: envía el mensaje msg a pid
func (ms *MessageSystem) Send(pid int, msg Message) {
	conn, err := net.Dial("tcp", ms.Peers[pid])
	checkError(err)
	var logMsg string
	if _, ok := msg.(*com.Request); ok {
		logMsg = "Requesting CS entrance"
	} else if rep, ok := msg.(*com.Reply); ok {
		if rep.AddedChar == "" {
			logMsg = "Allowing CS"
		} else {
			logMsg = "Updating file after write with " + rep.AddedChar
		}
	}
	vectorClockMessage := ms.logger.PrepareSend(logMsg, msg.ToBytes(), govec.GetDefaultLogOptions())

	// Send message
	_, err = conn.Write(vectorClockMessage)

	//encoder := gob.NewEncoder(conn)
	//err = encoder.Encode(&msg)

	com.CheckError(err)
	conn.Close()
}

// Pre: pid en {1..n}, el conjunto de procesos del SD
// Post: envía el mensaje msg a pid
func (ms *MessageSystem) SendAll(msg Message) {
	for i := 0; i < len(ms.Peers); i++ {
		if i != ms.Me {
			go ms.Send(i, msg)
		}

	}
}

// Pre: True
// Post: el mensaje msg de algún Proceso P_j se retira del mailbox y se devuelve
//
//	Si mailbox vacío, Receive bloquea hasta que llegue algún mensaje
func (ms *MessageSystem) Receive() (msg Message) {
	msg = <-ms.mbox
	return msg
}

//	messageTypes es un slice con tipos de mensajes que los procesos se pueden intercambiar a través de este ms
//
// Hay que registrar un mensaje antes de poder utilizar (enviar o recibir)
// Notar que se utiliza en la función New
func Register(messageTypes []Message) {
	for _, msgTp := range messageTypes {
		gob.Register(msgTp)
	}
}

// Pre: whoIam es el pid del proceso que inicializa este ms
//
//	usersFile es la ruta a un fichero de texto que en cada línea contiene IP:puerto de cada participante
//	messageTypes es un slice con todos los tipos de mensajes que los procesos se pueden intercambiar a través de este ms
func New(whoIam int, usersFile string, messageTypes []Message) (ms MessageSystem) {
	ms.Me = whoIam
	ms.Peers = parsePeers(usersFile)
	ms.mbox = make(chan Message, MAXMESSAGES)
	ms.done = make(chan bool)
	Register(messageTypes)

	//GoVector initialization
	logFilePath := fmt.Sprintf("./logs/actor%d", ms.Me)
	actorName := fmt.Sprintf("actor%d", ms.Me)

	logger := govec.InitGoVector(actorName, logFilePath, govec.GetDefaultConfig())
	opts := govec.GetDefaultLogOptions()
	ms.logger = logger
	ms.logOpts = opts

	go func() {
		listener, err := net.Listen("tcp", ms.Peers[ms.Me])
		checkError(err)
		fmt.Println("Process listening at " + ms.Peers[ms.Me])
		defer close(ms.mbox)
		for {
			select {
			case <-ms.done:
				return
			default:
				conn, err := listener.Accept()
				checkError(err)
				var buf = make([]byte, 1024)
				var encodedMessage = make([]byte, 1024)

				_, err = conn.Read(buf)
				com.CheckError(err)

				var decryptedMessage Message
				logger.UnpackReceive("Received message", buf, &encodedMessage, govec.GetDefaultLogOptions())

				decryptedMessage = FromBytes(encodedMessage)

				// Log a local event
				/*decoder := gob.NewDecoder(conn)

				err = decoder.Decode(&msg)
				com.CheckError(err)*/
				conn.Close()
				ms.mbox <- decryptedMessage
			}
		}
	}()
	return ms
}

// Pre: True
// Post: termina la ejecución de este ms
func (ms *MessageSystem) Stop() {
	ms.done <- true
}

// Returns our own endpoint
func (ms *MessageSystem) GetCurrentEndpoint() string {
	return ms.Peers[ms.Me]
}

func (ms *MessageSystem) LogLogicalEvent(event string) {
	ms.logger.LogLocalEvent(event, govec.GetDefaultLogOptions())
}

func FromBytes(data []byte) Message {
	// Read the first byte to identify the message type
	typeMarker := data[0]
	buf := bytes.NewBuffer(data[1:]) // Remove the type marker byte
	dec := gob.NewDecoder(buf)

	switch typeMarker {
	case 0x01: // It's a Request
		var req com.Request
		err := dec.Decode(&req)
		if err != nil {
			panic(err)
		}
		return &req
	case 0x02: // It's a Reply
		var rep com.Reply
		err := dec.Decode(&rep)
		if err != nil {
			panic(err)
		}
		return &rep
	default:
		panic("Unknown message type")
	}
}
