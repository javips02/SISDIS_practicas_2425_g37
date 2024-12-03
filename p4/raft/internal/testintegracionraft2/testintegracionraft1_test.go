package testintegracionraft1

import (
	"fmt"
	"raft/internal/comun/check"

	//"log"
	//"crypto/rand"

	"strconv"
	"testing"
	"time"

	"raft/internal/comun/rpctimeout"
	"raft/internal/despliegue"
	"raft/internal/raft"
)

const (
	//nodos replicas
	REPLICA1 = "127.0.0.1:29001"
	REPLICA2 = "127.0.0.1:29002"
	REPLICA3 = "127.0.0.1:29003"

	// paquete main de ejecutables relativos a directorio raiz de modulo
	EXECREPLICA = "cmd/srvraft/main.go"

	// comando completo a ejecutar en máquinas remota con ssh. Ejemplo :
	// 				cd $HOME/raft; go run cmd/srvraft/main.go 127.0.0.1:29001
)

// PATH de los ejecutables de modulo golang de servicio Raft
// var PATH string = filepath.Join(os.Getenv("HOME"), "tmp", "p3", "raft")
var PATH string = "/home/conte/Desktop/Lezioni/SSDD/SISDIS_practicas_2425_g37/p4/raft/"

// go run cmd/srvraft/main.go 0 127.0.0.1:29001 127.0.0.1:29002 127.0.0.1:29003
var EXECREPLICACMD string = "cd " + PATH + "; go run " + EXECREPLICA

//////////////////////////////////////////////////////////////////////////////
///////////////////////			 FUNCIONES TEST
/////////////////////////////////////////////////////////////////////////////

// TEST primer rango
func TestPrimerasPruebas(t *testing.T) { // (m *testing.M) {
	// <setup code>
	// Crear canal de resultados de ejecuciones ssh en maquinas remotas
	cfg := makeCfgDespliegue(t,
		3,
		[]string{REPLICA1, REPLICA2, REPLICA3},
		[]bool{true, true, true})

	// tear down code
	// eliminar procesos en máquinas remotas
	defer cfg.stop()

	// Run test sequence
	testPractica3 := false
	if testPractica3 {
		// Test1 : No debería haber ningun primario, si SV no ha recibido aún latidos
		t.Run("T1:soloArranqueYparada",
			func(t *testing.T) { cfg.soloArranqueYparadaTest1(t) })

		// Test2 : No debería haber ningun primario, si SV no ha recibido aún latidos
		t.Run("T2:ElegirPrimerLider",
			func(t *testing.T) { cfg.elegirPrimerLiderTest2(t) })

		// Test3: tenemos el primer primario correcto
		t.Run("T3:FalloAnteriorElegirNuevoLider",
			func(t *testing.T) { cfg.falloAnteriorElegirNuevoLiderTest3(t) })

		// Test4: Tres operaciones comprometidas en configuración estable
		t.Run("T4:tresOperacionesComprometidasEstable",
			func(t *testing.T) { cfg.tresOperacionesComprometidasEstable(t) })

		t.Run("T5:failComprometerNoLeader",
			func(t *testing.T) { cfg.failComprometerNoLeader(t) })
	}

	//t.Run("T6:comprometerConDosNodos",
	//	func(t *testing.T) { cfg.comprometerConDosNodos(t) })

	// Test4: Tres operaciones comprometidas en configuración estable
	//t.Run("T7:noComprometerConUnNodo",
	//	func(t *testing.T) { cfg.noComprometerConUnNodo(t) })

	t.Run("T8:someterConcorrentemente",
		func(t *testing.T) { cfg.someterConcorrentemente(t) })
}

// ---------------------------------------------------------------------
//
// Canal de resultados de ejecución de comandos ssh remotos
type canalResultados chan string

func (cr canalResultados) stop() {
	close(cr)

	// Leer las salidas obtenidos de los comandos ssh ejecutados
	for s := range cr {
		fmt.Println(s)
	}
}

// ---------------------------------------------------------------------
// Operativa en configuracion de despliegue y pruebas asociadas
type configDespliegue struct {
	t           *testing.T
	conectados  []bool
	numReplicas int
	nodosRaft   []rpctimeout.HostPort
	cr          canalResultados
}

// Crear una configuracion de despliegue
func makeCfgDespliegue(t *testing.T, n int, nodosraft []string,
	conectados []bool) *configDespliegue {
	cfg := &configDespliegue{}
	cfg.t = t
	cfg.conectados = conectados
	cfg.numReplicas = n
	cfg.nodosRaft = rpctimeout.StringArrayToHostPortArray(nodosraft)
	cfg.cr = make(canalResultados, 2000)

	return cfg
}

func (cfg *configDespliegue) stop() {
	//cfg.stopDistributedProcesses()

	time.Sleep(50 * time.Millisecond)

	cfg.cr.stop()
}

// --------------------------------------------------------------------------
// FUNCIONES DE SUBTESTS

// Se ponen en marcha replicas - 3 NODOS RAFT
func (cfg *configDespliegue) soloArranqueYparadaTest1(t *testing.T) {
	//t.Skip("SKIPPED soloArranqueYparadaTest1")

	// Parar réplicas almacenamiento en remoto
	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.t = t // Actualizar la estructura de datos de tests para errores

	// Poner en marcha replicas en remoto con un tiempo de espera incluido
	cfg.startDistributedProcesses(true)

	// Comprobar estado replica 0
	cfg.comprobarEstadoRemoto(0, 0, false, -1)

	// Comprobar estado replica 1
	cfg.comprobarEstadoRemoto(1, 0, false, -1)

	// Comprobar estado replica 2
	cfg.comprobarEstadoRemoto(2, 0, false, -1)

	fmt.Println(".............", t.Name(), "Superado")
}

// Primer lider en marcha - 3 NODOS RAFT
func (cfg *configDespliegue) elegirPrimerLiderTest2(t *testing.T) {
	//t.Skip("SKIPPED ElegirPrimerLiderTest2")

	// Parar réplicas almacenamiento en remoto

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	// Se ha elegido lider ?
	fmt.Printf("Probando lider en curso\n")
	cfg.pruebaUnLider(3)

	fmt.Println(".............", t.Name(), "Superado")
	defer cfg.stopDistributedProcesses() //parametros
}

// Fallo de un primer lider y reeleccion de uno nuevo - 3 NODOS RAFT
func (cfg *configDespliegue) falloAnteriorElegirNuevoLiderTest3(t *testing.T) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	// Parar réplicas almacenamiento en remoto
	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	fmt.Printf("Lider inicial\n")
	idLider := cfg.pruebaUnLider(3)

	var reply raft.Vacio
	fmt.Printf("Apagando leader, nodo %d\n", idLider)
	err := cfg.nodosRaft[idLider].CallTimeout("NodoRaft.ParaNodo",
		raft.Vacio{}, &reply, 10*time.Millisecond)
	check.CheckError(err, "Error al parar primero leader")

	time.Sleep(1500 * time.Millisecond)

	fmt.Printf("Comprobar nuevo lider\n")
	cfg.pruebaUnLider(3)

	fmt.Println(".............", t.Name(), "Superado")
}

// 4 operaciones comprometidas con situacion estable y sin fallos - 3 NODOS RAFT
func (cfg *configDespliegue) tresOperacionesComprometidasEstable(t *testing.T) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	idLider := cfg.pruebaUnLider(3)
	fmt.Printf("Lider inicial es %d\n", idLider)

	var someterReply raft.ResultadoRemoto

	op1 := raft.Entry{
		Operation: "write",
		Key:       "hola-it",
		Value:     "ciao",
	}
	op2 := raft.Entry{
		Operation: "write",
		Key:       "hola-en",
		Value:     "hello",
	}
	op3 := raft.Entry{
		Operation: "read",
		Key:       "hola-en",
	}
	op4 := raft.Entry{
		Operation: "read",
		Key:       "hola-it",
	}

	err := cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op1, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 1")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida\n")
		cfg.t.Fail()
	}

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op2, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 2")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida")
		cfg.t.Fail()
	}

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op3, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 3")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida")
		cfg.t.Fail()
	}

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op4, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 4")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida")
		cfg.t.Fail()
	}

	// Parar réplicas almacenamiento en remoto

	fmt.Println(".............", t.Name(), "Superado")
}

// Comprobamos que no se pueda comprometer una entrada pidiendo a un nodo seguidor
func (cfg *configDespliegue) failComprometerNoLeader(t *testing.T) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	idLider := cfg.pruebaUnLider(3)
	fmt.Printf("Lider inicial es %d\n", idLider)
	nodosNoLeader := []int{(idLider + 1) % 3, (idLider + 2) % 3}

	var someterReply raft.ResultadoRemoto

	op1 := raft.Entry{
		Operation: "write",
		Key:       "hola-it",
		Value:     "ciao",
	}
	for _, idNodoNoLeader := range nodosNoLeader {
		err := cfg.nodosRaft[idNodoNoLeader].CallTimeout("NodoRaft.SometerOperacionRaft",
			&op1, &someterReply, 1000*time.Millisecond)
		check.CheckError(err, "Error al someter operaciòn 1")
		if someterReply.Success ||
			someterReply.EsLider == true ||
			someterReply.IdLider != idLider {
			fmt.Printf("Nodo %d intento comprometer entrada o tiene estado incorrecto\n", idNodoNoLeader)
			fmt.Println(someterReply)
			t.Fail()
		}
	}

	// Parar réplicas almacenamiento en remoto

	fmt.Println(".............", t.Name(), "Superado")
}

func (cfg *configDespliegue) comprometerConDosNodos(t *testing.T) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	idLider := cfg.pruebaUnLider(3)
	fmt.Printf("Lider inicial es %d\n", idLider)
	nodoNoLeader := (idLider + 1) % 3

	var someterReply raft.ResultadoRemoto

	op1 := raft.Entry{
		Operation: "write",
		Key:       "hola-it",
		Value:     "ciao",
	}

	op2 := raft.Entry{
		Operation: "write",
		Key:       "hola-en",
		Value:     "hello",
	}

	var reply raft.Vacio
	fmt.Printf("Apagando seguidor, nodo %d\n", nodoNoLeader)
	err := cfg.nodosRaft[nodoNoLeader].CallTimeout("NodoRaft.ParaNodo",
		raft.Vacio{}, &reply, 10*time.Millisecond)
	check.CheckError(err, "Error al parar primero leader")

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op1, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 1")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida\n")
		cfg.t.Fail()
	}

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op2, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 2")
	if !(someterReply.Success) {
		fmt.Printf("Operaciòn no comprometida")
		cfg.t.Fail()
	}
	// Parar réplicas almacenamiento en remoto

	fmt.Println(".............", t.Name(), "Superado")
}
func (cfg *configDespliegue) noComprometerConUnNodo(t *testing.T) {
	//t.Skip("SKIPPED noComprometerConUnNodo")

	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	idLider := cfg.pruebaUnLider(3)
	fmt.Printf("Lider inicial es %d\n", idLider)

	var someterReply raft.ResultadoRemoto

	op1 := raft.Entry{
		Operation: "write",
		Key:       "hola-it",
		Value:     "ciao",
	}

	op2 := raft.Entry{
		Operation: "write",
		Key:       "hola-en",
		Value:     "hello",
	}

	var reply raft.Vacio

	//Apagando los dos nodos no leader
	nodosNoLeader := []int{(idLider + 1) % 3, (idLider + 2) % 3}
	for _, idNodoNoLeader := range nodosNoLeader {
		err := cfg.nodosRaft[idNodoNoLeader].CallTimeout("NodoRaft.ParaNodo",
			raft.Vacio{}, &reply, 10*time.Millisecond)
		check.CheckError(err, "Error al parar seguidor")
	}

	time.Sleep(5 * time.Second)

	err := cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op1, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 1")
	if someterReply.Success {
		fmt.Printf("Operaciòn comprometida por error\n")
		cfg.t.Fail()
	}

	err = cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
		&op2, &someterReply, 1000*time.Millisecond)
	check.CheckError(err, "Error al someter operaciòn 2")
	if someterReply.Success {
		fmt.Printf("Operaciòn comprometida por error\n")
		cfg.t.Fail()
	}
	// Parar réplicas almacenamiento en remoto

	fmt.Println(".............", t.Name(), "Superado")
}
func (cfg *configDespliegue) someterConcorrentemente(t *testing.T) {
	//t.Skip("SKIPPED FalloAnteriorElegirNuevoLiderTest3")

	defer cfg.stopDistributedProcesses() //parametros

	fmt.Println(t.Name(), ".....................")

	cfg.startDistributedProcesses(false)

	idLider := cfg.pruebaUnLider(3)
	fmt.Printf("Lider inicial es %d\n", idLider)

	var someterReply raft.ResultadoRemoto

	op1 := raft.Entry{
		Operation: "write",
		Key:       "hola-it",
		Value:     "ciao",
	}
	op2 := raft.Entry{
		Operation: "write",
		Key:       "hola-en",
		Value:     "hello",
	}
	op3 := raft.Entry{
		Operation: "read",
		Key:       "hola-en",
	}
	op4 := raft.Entry{
		Operation: "read",
		Key:       "hola-it",
	}
	op5 := raft.Entry{
		Operation: "write",
		Key:       "ultima",
		Value:     "operazione",
	}

	entries := []raft.Entry{
		op1,
		op2,
		op3,
		op4,
		op5,
	}

	chanWait := make(chan (raft.Vacio), 5)
	for index, entry := range entries {
		go func(index int, entry raft.Entry, chanWait chan raft.Vacio) {
			err := cfg.nodosRaft[idLider].CallTimeout("NodoRaft.SometerOperacionRaft",
				&entry, &someterReply, 1000*time.Millisecond)
			check.CheckError(err, "Error al someter operaciòn")
			if !(someterReply.Success) {
				fmt.Printf("Operaciòn no comprometida\n")
				cfg.t.Fail()
			}
			chanWait <- raft.Vacio{}
		}(index, entry, chanWait)
	}

	for i := 0; i < 5; i++ {
		<-chanWait
	}
	_, _, _, _, commitIndex, _ := cfg.obtenerEstadoRemoto(idLider)

	if commitIndex != 5 {
		t.Fail()
		fmt.Printf("commitIndex = %d; expected 5\n", commitIndex)
	}

	fmt.Println(".............", t.Name(), "Superado")
}

// --------------------------------------------------------------------------
// FUNCIONES DE APOYO
// --------------------------------------------------------------------------

// Comprobar que hay un solo lider
// probar varias veces si se necesitan reelecciones
func (cfg *configDespliegue) pruebaUnLider(numreplicas int) int {
	for iters := 0; iters < 10; iters++ {
		time.Sleep(500 * time.Millisecond)
		mapaLideres := make(map[int][]int)
		for i := 0; i < numreplicas; i++ {
			if cfg.conectados[i] {
				if _, mandato, eslider, _, _, _ := cfg.obtenerEstadoRemoto(i); eslider {
					mapaLideres[mandato] = append(mapaLideres[mandato], i)
				}
			}
		}

		ultimoMandatoConLider := -1
		for mandato, lideres := range mapaLideres {
			if len(lideres) > 1 {
				cfg.t.Fail()
				fmt.Printf("mandato %d tiene %d (>1) lideres\n",
					mandato, len(lideres))
			}
			if mandato > ultimoMandatoConLider {
				ultimoMandatoConLider = mandato
			}
		}

		if len(mapaLideres) != 0 {

			return mapaLideres[ultimoMandatoConLider][0] // Termina

		}
	}
	fmt.Println("un lider esperado, ninguno obtenido")
	cfg.t.Fail()

	return -1 // Termina
}

func (cfg *configDespliegue) obtenerEstadoRemoto(
	indiceNodo int) (int, int, bool, int, int, int) {

	var reply = raft.EstadoRemoto{IdNodo: 0, EstadoParcial: raft.EstadoParcial{}}

	err := cfg.nodosRaft[indiceNodo].CallTimeout("NodoRaft.ObtenerEstadoNodo",
		&raft.Vacio{}, &reply, 10*time.Millisecond)

	check.CheckError(err, "Error en llamada RPC ObtenerEstadoRemoto")
	if err != nil {
		fmt.Println("Error en llamada RPC ObtenerEstadoRemoto", err)
	}

	return reply.IdNodo, reply.Mandato, reply.EsLider,
		reply.IdLider, reply.CommitIndex, reply.LastApplied
}

// start  gestor de vistas; mapa de replicas y maquinas donde ubicarlos;
// y lista clientes (host:puerto)
func (cfg *configDespliegue) startDistributedProcesses(shouldWait bool) {
	//cfg.t.Log("Before start following distributed processes: ", cfg.nodosRaft)
	shouldWaitInt := 0
	if shouldWait {
		shouldWaitInt = 1
	}
	for i, endPoint := range cfg.nodosRaft {
		despliegue.ExecMutipleHosts(EXECREPLICACMD+
			" "+strconv.Itoa(i)+" "+strconv.Itoa(shouldWaitInt)+" "+
			rpctimeout.HostPortArrayToString(cfg.nodosRaft),
			[]string{endPoint.Host()}, cfg.cr)

		// dar tiempo para se establezcan las replicas
		time.Sleep(3000 * time.Millisecond)
	}

	// aproximadamente 500 ms para cada arranque por ssh en portatil
	time.Sleep(2000 * time.Millisecond)
}

func (cfg *configDespliegue) stopDistributedProcesses() {
	var reply raft.Vacio

	for _, endPoint := range cfg.nodosRaft {
		err := endPoint.CallTimeout("NodoRaft.ParaNodo",
			raft.Vacio{}, &reply, 10*time.Millisecond)
		check.CheckError(err, "Error en llamada RPC Para nodo")
	}
}

// Comprobar estado remoto de un nodo con respecto a un estado prefijado
func (cfg *configDespliegue) comprobarEstadoRemoto(idNodoDeseado int,
	mandatoDeseado int, esLiderDeseado bool, IdLiderDeseado int) {
	idNodo, mandato, esLider, idLider, _, _ := cfg.obtenerEstadoRemoto(idNodoDeseado)

	//cfg.t.Log("Estado replica 0: ", idNodo, mandato, esLider, idLider, "\n")

	if idNodo != idNodoDeseado || mandato != mandatoDeseado ||
		esLider != esLiderDeseado || idLider != IdLiderDeseado {
		fmt.Printf("Estado incorrecto en replica %d en subtest %s\n",
			idNodoDeseado, cfg.t.Name())
		cfg.t.Fail()
	}

}
