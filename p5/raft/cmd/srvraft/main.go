package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"raft/internal/comun/check"
	"raft/internal/comun/rpctimeout"
	"raft/internal/raft"
	"regexp"
	"strconv"
)

func main() {
	// Regex para capturar el índice del nodo (número después de "ss-")
	regex := regexp.MustCompile(`ss-(\d+)\.`)

	// Extraer el índice del nodo de `os.Args[1]`
	matches := regex.FindStringSubmatch(os.Args[1])
	if len(matches) < 2 {
		fmt.Println("Error: No se pudo extraer el índice del nodo de:", os.Args[1])
		os.Exit(1)
	}
	// Convertir el índice extraído a entero
	me, err := strconv.Atoi(matches[1])
	if err != nil {
		fmt.Println("Error: Índice de nodo no es un entero válido:", matches[1])
		os.Exit(1)
	}
	fmt.Println("Índice del nodo:", me)

	// para tests (introducido en la práctica 4 para que todos los nodos
	// se inicien a la vez)
	shouldWait, err := strconv.Atoi(os.Args[2])
	check.CheckError(err, "Main, error en ShouldWait:")

	var nodos []rpctimeout.HostPort
	// Resto de argumento son los end points como strings
	// De todas la replicas-> pasarlos a HostPort
	for _, endPoint := range os.Args[3:] {
		nodos = append(nodos, rpctimeout.HostPort(endPoint))
	}

	// Parte Servidor
	nr := raft.NuevoNodo(nodos, me, shouldWait, make(chan raft.AplicaOperacion, 1000))
	rpc.Register(nr)

	fmt.Println("Replica escucha en :", me, " de ", os.Args[3:])
	fmt.Println("Osea en :", os.Args[3:][me])
	l, err := net.Listen("tcp", os.Args[3:][me])
	check.CheckError(err, "Main listen error:")

	rpc.Accept(l)
}
