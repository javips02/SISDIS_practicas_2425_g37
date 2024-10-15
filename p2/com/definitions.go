/*
* AUTOR: Rafael Tolosana Calasanz y Unai Arronategui
* ASIGNATURA: 30221 Sistemas Distribuidos del Grado en Ingeniería Informática
*			Escuela de Ingeniería y Arquitectura - Universidad de Zaragoza
* FECHA: septiembre de 2022
* FICHERO: definitions.go
* DESCRIPCIÓN: contiene las definiciones de estructuras de datos necesarias para
*			la práctica 1
 */
package com

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"
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

func CheckError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
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
