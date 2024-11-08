// Escribir vuestro código de funcionalidad Raft en este fichero
//

package raft

//
// API
// ===
// Este es el API que vuestra implementación debe exportar
//
// nodoRaft = NuevoNodo(...)
//   Crear un nuevo servidor del grupo de elección.
//
// nodoRaft.Para()
//   Solicitar la parado de un servidor
//
// nodo.ObtenerEstado() (yo, mandato, esLider)
//   Solicitar a un nodo de elección por "yo", su mandato en curso,
//   y si piensa que es el msmo el lider
//
// nodoRaft.SometerOperacion(operacion interface()) (indice, mandato, esLider)

// type AplicaOperacion

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/rpc"
	"os"
	"strconv"

	"sync"
	"time"

	//"net/rpc"

	"raft/internal/comun/rpctimeout"
)

const (
	// Constante para fijar valor entero no inicializado
	IntNOINICIALIZADO = -1

	//  false deshabilita por completo los logs de depuracion
	// Aseguraros de poner kEnableDebugLogs a false antes de la entrega
	kEnableDebugLogs = true

	// Poner a true para logear a stdout en lugar de a fichero
	kLogToStdout = false

	// Cambiar esto para salida de logs en un directorio diferente
	kLogOutputDir = "./logs_raft/"
)

type State string

// Define the enum values
const (
	Leader    State = "leader"
	Candidate State = "candidate"
	Follower  State = "follower"
)

type Operacion struct {
	Operacion string // La operaciones posibles son "leer" y "escribir"
	Clave     string
	Valor     string // en el caso de la lectura Valor = ""
}

// A medida que el nodo Raft conoce las operaciones de las  entradas de registro
// comprometidas, envía un AplicaOperacion, con cada una de ellas, al canal
// "canalAplicar" (funcion NuevoNodo) de la maquina de estados
type AplicaOperacion struct {
	Indice    int // en la entrada de registro
	Operacion Operacion
}

// Tipo de dato Go que representa un solo nodo (réplica) de raft
type NodoRaft struct {
	mutex sync.Mutex // Mutex para proteger acceso a estado compartido
	// Host:Port de todos los nodos (réplicas) Raft, en mismo orden
	Nodos   []rpctimeout.HostPort
	Yo      int // indice de este nodos en campo array "nodos"
	IdLider int
	State   State

	//Time before the replicas should consider a timeout and start election
	timeoutTime time.Duration
	//Time before the leader should send a heartbeat
	heartbeatTime time.Duration

	// Utilización opcional de este logger para depuración
	// Cada nodo Raft tiene su propio registro de trazas (logs)
	Logger  *log.Logger
	Entries map[string]string
	// Vuestros datos aqui.

	// VALORES PERSISTENTES EN TODOS LOS SERVIDORES //

	mandatoActual int // Indica el mandato más reciente que esta réplica conoce
	votedFor      int //candidato que ha recibido el voto en el mandato actual
	//numVotes      int //number of votes recieved in the current election
	//el log es Entries map[string][string] que venía dado

	// VALORES VOLÁTILES DE ESTADO EN TODOS LOS SERVIDORES //

	commitIndex  int         // valor más alto de entrada comprometida por esta réplica (0...)
	lastApplied  int         // índice de la entrada más alta aplicada a nuestra máquina de estados (0 ...)
	timeoutTimer *time.Timer // tiempo restante para iniciar una nueva eleccion (seguidores)
	// VALORES VOLÁTILES EN RÉPLICAS LÍDER (reinicializar estos valores después de cada elección) //

	nextIndex             []int        //en cada posición, el índice de la siguiente entrada a mandar al servidor (leader_lastLog+1...)
	matchIndex            []int        //índice de la entrada más alta que cada server sabe que está replicada en sí mismo (0...)
	leaderHeartBeatTicker *time.Ticker //tiempo restante para dar un nuevo latido
}

var barrera sync.WaitGroup

// Creacion de un nuevo nodo de eleccion
//
// Tabla de <Direccion IP:puerto> de cada nodo incluido a si mismo.
//
// <Direccion IP:puerto> de este nodo esta en nodos[yo]
//
// Todos los arrays nodos[] de los nodos tienen el mismo orden

// canalAplicar es un canal donde, en la practica 5, se recogerán las
// operaciones a aplicar a la máquina de estados. Se puede asumir que
// este canal se consumira de forma continúa.
//
// NuevoNodo() debe devolver resultado rápido, por lo que se deberían
// poner en marcha Gorutinas para trabajos de larga duracion
func NuevoNodo(nodos []rpctimeout.HostPort, yo int,
	canalAplicarOperacion chan AplicaOperacion) *NodoRaft {
	nr := &NodoRaft{}
	nr.Nodos = nodos
	nr.Yo = yo
	nr.mandatoActual = 0
	nr.IdLider = -1
	nr.State = Follower
	barrera.Add(1)

	if kEnableDebugLogs {
		nombreNodo := nodos[yo].Host() + "_" + nodos[yo].Port()
		log.Println("nombreNodo: ", nombreNodo)

		if kLogToStdout {
			nr.Logger = log.New(os.Stdout, nombreNodo+" -->> ",
				log.Lmicroseconds|log.Lshortfile)
		} else {
			err := os.MkdirAll(kLogOutputDir, os.ModePerm)
			if err != nil {
				nr.Logger.Println(err.Error())
				panic(err.Error())
			}
			logOutputFile, err := os.OpenFile(
				fmt.Sprintf("%s/%s.txt", kLogOutputDir, nombreNodo),
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				0755)
			if err != nil {
				nr.Logger.Println(err.Error())
				panic(err.Error())
			}
			nr.Logger = log.New(logOutputFile,
				nombreNodo+" -> ", log.Lmicroseconds|log.Lshortfile)
		}
		nr.Logger.Println("logger initialized")
	} else {
		nr.Logger = log.New(io.Discard, "", 0)
	}

	// Añadir codigo de inicialización
	// Inicialización de otros campos
	nr.Entries = make(map[string]string) // Mapa vacío para las entradas
	nr.votedFor = -1                     // No ha votado aún
	//nr.logEntries = []string{}           // Inicialmente sin entradas de log
	nr.commitIndex = 0 // Sin entradas comprometidas aún
	nr.lastApplied = 0 // Ninguna entrada aplicada aún

	// Inicialización para líder (valores volátiles)
	numNodos := len(nodos)                // Número total de nodos
	nr.nextIndex = make([]int, numNodos)  // Inicializar nextIndex en cada nodo
	nr.matchIndex = make([]int, numNodos) // Inicializar matchIndex en cada nodo

	// Asignar los valores iniciales para nextIndex y matchIndex
	for i := range nr.nextIndex {
		nr.nextIndex[i] = 1  // El índice siguiente entrada a enviar (init a 1)
		nr.matchIndex[i] = 0 // Ninguna entrada ha sido replicada aún
	}


	go nr.monitorizarTemporizadoresRaft() // monitorizar timeout eleccion y HB
	//IdNodo, Mandato, EsLider, IdLider := nr.obtenerEstado()
	nr.Logger.Println(nr.obtenerEstado())
	return nr
}

// Metodo randomElectionTimeout() utilizado cuando queremos asignar un tiempo
// limite sin lider dentor de cada nodo. Devuelve un tiempo aleatorio en ms
// entre 200 y 400
func randomElectionTimeout() time.Duration {
	return time.Duration(500+rand.Intn(600)) * time.Millisecond
}

func heartbeatTimeout() time.Duration {
	return time.Duration(50) * time.Millisecond
}

// Metodo Para() utilizado cuando no se necesita mas al nodo
//
// Quizas interesante desactivar la salida de depuracion
// de este nodo
func (nr *NodoRaft) para() {
	go func() { time.Sleep(5 * time.Millisecond); os.Exit(0) }()
}

// Devuelve "yo", mandato en curso y si este nodo cree ser lider
//
// Primer valor devuelto es el indice de este  nodo Raft el el conjunto de nodos
// la operacion si consigue comprometerse.
// El segundo valor es el mandato en curso
// El tercer valor es true si el nodo cree ser el lider
// Cuarto valor es el lider, es el indice del líder si no es él
func (nr *NodoRaft) obtenerEstado() (int, int, bool, int) {

	nr.mutex.Lock()
	var yo = nr.Yo
	esLider := nr.IdLider == nr.Yo
	idLider := nr.IdLider
	mandato := 0

	nr.mutex.Unlock()

	return yo, mandato, esLider, idLider
}

// El servicio que utilice Raft (base de datos clave/valor, por ejemplo)
// Quiere buscar un acuerdo de posicion en registro para siguiente operacion
// solicitada por cliente.
//
// Si el nodo no es el lider, devolver falso
// Sino, comenzar la operacion de consenso sobre la operacion y devolver en
// cuanto se consiga
//
// No hay garantía que esta operación consiga comprometerse en una entrada de
// de registro, dado que el lider puede fallar y la entrada ser reemplazada
// en el futuro.
// Resultado de este method :
// - Primer valor devuelto es el indice del registro donde se va a colocar
// - la operacion si consigue comprometerse.
// - El segundo valor es el mandato en curso
// - El tercer valor es true si el nodo cree ser el lider
// - Cuarto valor es el lider, es el indice del líder si no es él
// - Quinto valor es el resultado de aplicar esta operación en máquina de estados
func (nr *NodoRaft) someterOperacion(operacion Operacion) (int, int,
	bool, int, string) {

	nr.mutex.Lock()
	indice := nr.commitIndex
	mandato := 0 //TODO: cambiar en la siguiente práctica
	EsLider := nr.IdLider == nr.Yo
	idLider := nr.IdLider
	nr.mutex.Unlock()

	valorADevolver := ""

	// no lider => devolver falso (incluye quién es lider en la respuesta, el cliente tiene que reenviar
	if !EsLider {
		valorADevolver = "false"
		return indice, mandato, EsLider, idLider, valorADevolver
	}

	nr.Logger.Println(operacion)
	// Definir la operación a someter
	entrada := Entry{
		op: operacion, // Aquí `operacion` es el parámetro que recibes en `someterOperacion`

	}

	var comprometidos int = 0
	//barrera:
	var wg sync.WaitGroup // WaitGroup para esperar a que todas las goroutines terminen

	for i := 0; i < len(nr.Nodos); i++ {
		wg.Add(1) // Incrementar el contador de goroutines pendientes

		go func(peer rpctimeout.HostPort, nr *NodoRaft, comprometidos *int) {
			defer wg.Done() // Decrementar el contador de goroutines pendientes al finalizar la goroutine
			client, err := rpc.DialHTTP("tcp", "localhost"+":2233")
			if err != nil {
				nr.Logger.Println(err.Error())

				log.Fatal("dialing:", err)
			}
			//TODO: debería meter todas las entradas que no están sincronizadas con un bucle?
			args := ArgAppendEntries{Entries: []Entry{entrada}} //meter entrada/s para comprometer
			reply := Results{}
			err = client.Call("NodoRaft.AppendEntries", args, &reply)
			if err != nil {
				nr.Logger.Println(err.Error())
				log.Fatal("arith error:", err)
			}
			// si se ha comprometido la entrada en el nodo i,
			// aumentar el contador de forma atómica
			if reply.success {

				nr.mutex.Lock()
				*comprometidos++
				nr.mutex.Unlock()
			}
		}(nr.Nodos[i], nr, &comprometidos)
	}
	// esperar a que todos los subprocesos alcancen la barrera
	// claro, no tengo que esperar a todos... apañar esto TODO
	wg.Wait()

	//caso de exito al comprometer la entrada
	if comprometidos >= len(nr.Nodos)/2 {
		valorADevolver = operacion.Operacion
	} else {
		valorADevolver = "false"
	}

	return indice, mandato, EsLider, idLider, valorADevolver
}

// -----------------------------------------------------------------------
// LLAMADAS RPC al API
//
// Si no tenemos argumentos o respuesta estructura vacia (tamaño cero)
type Vacio struct{}

func (nr *NodoRaft) ParaNodo(args Vacio, reply *Vacio) error {
	defer nr.para()
	return nil
}

type EstadoParcial struct {
	Mandato int
	EsLider bool
	IdLider int
}

type EstadoRemoto struct {
	IdNodo int
	EstadoParcial
}

func (nr *NodoRaft) ObtenerEstadoNodo(args Vacio, reply *EstadoRemoto) error {
	reply.IdNodo, reply.Mandato, reply.EsLider, reply.IdLider = nr.obtenerEstado()
	nr.Logger.Println(nr.obtenerEstado())
	return nil
}

type ResultadoRemoto struct {
	ValorADevolver string
	IndiceRegistro int
	EstadoParcial
}

func (nr *NodoRaft) SometerOperacionRaft(operacion *Operacion,
	reply *ResultadoRemoto) error {
	reply.IndiceRegistro, reply.Mandato, reply.EsLider,
		reply.IdLider, reply.ValorADevolver = nr.someterOperacion(*operacion)
	return nil
}

// -----------------------------------------------------------------------
// LLAMADAS RPC protocolo RAFT
//
// Structura de ejemplo de argumentos de RPC PedirVoto.
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type ArgsPeticionVoto struct {
	// Vuestros datos aqui
	Mandato      int //(para pr4)
	CandidateId  int //candidato pidiendo el voto
	LastLogIndex int // indice de la ultima entrada del log del candidato
	//lastLogTerm int (para pr4)
}

// Structura de ejemplo de respuesta de RPC PedirVoto,
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type RespuestaPeticionVoto struct {
	// Vuestros datos aqui
	// mandato ...
	VoteGranted bool
}

// Reset heartbeat counter
func (nr *NodoRaft) Heartbeat(args *HeartbeatArgs,
	output *Vacio) error {
	nr.Logger.Println("PUM")
	nr.timeoutTimer.Reset(nr.timeoutTime)
	nr.mutex.Lock()
	if nr.mandatoActual < args.Mandato {
		nr.mandatoActual = args.Mandato
		nr.IdLider = args.IdLeader
		nr.Logger.Println("Tocado leader")
		nr.State = Follower
	}
	nr.mutex.Unlock()
	return nil
}

// Metodo para RPC PedirVoto
func (nr *NodoRaft) PedirVoto(peticion *ArgsPeticionVoto,
	reply *RespuestaPeticionVoto) error {

	nr.mutex.Lock()
	//A process remains a follower as long as he gets RPCs from leaders or candidates
	nr.timeoutTimer.Reset(nr.timeoutTime)
	if peticion.Mandato > nr.mandatoActual {
		nr.mandatoActual = peticion.Mandato
		nr.votedFor = -1
	}

	nr.Logger.Print("Got PedirVoto by ", peticion.CandidateId)
	if nr.votedFor == -1 {
		nr.votedFor = peticion.CandidateId
		reply.VoteGranted = true
	} else {
		reply.VoteGranted = false
	}
	nr.mutex.Unlock()

	if reply.VoteGranted {
		nr.Logger.Print("Granted vote request to ", peticion.CandidateId)
	} else {
		nr.Logger.Print("Denied vote request to ", peticion.CandidateId)
	}
	nr.Logger.Println(", voted for: ", nr.votedFor)

	return nil
}

type Entry struct {
	op    Operacion
	index int
}
type ArgAppendEntries struct {
	Entries      []Entry
	LeaderCommit int // index del commit para el vector del líder
	// añadir term, leadirId, precLogIndex, prevLogTerm si necesario
}

type HeartbeatArgs struct {
	IdLeader int
	Mandato  int
}

type Results struct {
	success bool
}

// El metodo que el leader llama en los seguidores para insertar una nueva entrada en los seguidores
// Pueden insertarse varias entradas de un paso, por ejemplo cuando el nodo revive despues de un fallo :)
func (nr *NodoRaft) AppendEntries(args *ArgAppendEntries,
	results *Results) error {
	results.success = true //sin mandatos siempre será success
	// if term < currentTerm --> reply false
	// if !exists entry at prevLogIndex == term from prevLogTerm --> reply false (outdated)
	//if newEntry.index == otherEntry.index && termNew != termOther --> reply false

	//Append any new entries not already in the log

	nr.mutex.Lock()
	for key, value := range args.Entries {
		nr.Entries[strconv.Itoa(key)] = value.op.Operacion //meter el comando con su índice
	}
	nr.mutex.Unlock()

	// if leader commit > commit index form current node, choose min
	if args.LeaderCommit > nr.commitIndex {
		nr.commitIndex = min(args.LeaderCommit, nr.commitIndex)
	}
	return nil //si llega hasta aquí, return NoError (error nil) de RPC
}

// --------------------------------------------------------------------------
// ----- METODOS/FUNCIONES desde nodo Raft, como cliente, a otro nodo Raft
// --------------------------------------------------------------------------

// Ejemplo de código enviarPeticionVoto
//
// nodo int -- indice del servidor destino en nr.nodos[]
//
// args *RequestVoteArgs -- argumentos par la llamada RPC
//
// reply *RequestVoteReply -- respuesta RPC
//
// Los tipos de argumentos y respuesta pasados a CallTimeout deben ser
// los mismos que los argumentos declarados en el metodo de tratamiento
// de la llamada (incluido si son punteros)
//
// Si en la llamada RPC, la respuesta llega en un intervalo de tiempo,
// la funcion devuelve true, sino devuelve false
//
// la llamada RPC deberia tener un timeout adecuado.
//
// Un resultado falso podria ser causado por una replica caida,
// un servidor vivo que no es alcanzable (por problemas de red ?),
// una petición perdida, o una respuesta perdida
//
// Para problemas con funcionamiento de RPC, comprobar que la primera letra
// del nombre de todos los campos de la estructura (y sus subestructuras)
// pasadas como parametros en las llamadas RPC es una mayuscula,
// Y que la estructura de recuperacion de resultado sea un puntero a estructura
// y no la estructura misma.
func (nr *NodoRaft) enviarPeticionVoto(nodo int, args *ArgsPeticionVoto,
	reply *RespuestaPeticionVoto) bool {

	// Completar con la llamada RPC correcta incluida
	// Configura el host del nodo destino para la llamada RPC.
	peer := nr.Nodos[nodo]
	// Llamada RPC con timeout
	err := peer.CallTimeout("NodoRaft.PedirVoto", args, reply, 200*time.Millisecond) // TODO: ajustar timeout
	if err != nil {
		//nr.Logger.Println("Error al enviar petición de voto:", err)
		return false
	}

	nr.Logger.Println(nodo, args, reply)
	if !reply.VoteGranted {
		nr.Logger.Println(nodo, " me denegò voto a mi, ", nr.Yo)
		return false
	}
	nr.Logger.Println(nodo, " me diò voto a mi, ", nr.Yo)
	return true
}

// --------------------------------------------------------------------------
// ----- METODOS/FUNCIONES de temporizacion y eleccion de lider
// --------------------------------------------------------------------------

func (nr *NodoRaft) monitorizarTemporizadoresRaft() {
<<<<<<< HEAD
	nr.Logger.Println("Started monitorizar")
=======
	// Inicializacion de timers
	time.Sleep(1 * time.Second)
	nr.timeoutTime = randomElectionTimeout()
	nr.heartbeatTime = heartbeatTimeout()

	fmt.Println("Election timeout: ", nr.timeoutTime, "ms")
	fmt.Println("Heartbeat interval: ", nr.timeoutTime, "ms")

	nr.timeoutTimer = time.NewTimer(nr.timeoutTime)
	nr.leaderHeartBeatTicker = time.NewTicker(nr.heartbeatTime)
	nr.leaderHeartBeatTicker.Stop()
	//So that channel is created at least

	fmt.Println("Started monitorizar")
>>>>>>> 0dd7bff0d1dbe510353e858737a20cec889e1353
	for {
		select {
		//In this case, leader hasn't sent a heartbeat in a while, so we start eection
		case <-nr.timeoutTimer.C: // Election timeout case
			nr.mutex.Lock()
			amLeader := nr.IdLider == nr.Yo
			nr.mutex.Unlock()

			if !amLeader { // If I'm not the leader
				nr.Logger.Println("Election!")
				nr.iniciarEleccion() // Start election
			}

		case <-nr.leaderHeartBeatTicker.C: // Leader heartbeat case
			nr.mutex.Lock()
			amLeader := nr.IdLider == nr.Yo
			nr.mutex.Unlock()

			if amLeader { // If I am the leader
				nr.enviarLatidosATodos()
			}
		}
	}
}

func (nr *NodoRaft) enviarLatidosATodos() {
	args := HeartbeatArgs{
		IdLeader: nr.Yo,
		Mandato:  nr.mandatoActual,
	}
	for i := 0; i < len(nr.Nodos); i++ {
		go func(nr *NodoRaft, nodo int, args *HeartbeatArgs) {
			reply := Vacio{}
			if nodo != nr.Yo {
				err := nr.Nodos[nodo].CallTimeout("NodoRaft.Heartbeat", args, &reply, 10*time.Millisecond)
				if err != nil {
					//The node is down, we ignore
					//nr.Logger.Println("Error akì", err.Error())
				}
			}
		}(nr, i, &args)
	}
}

func (nr *NodoRaft) iniciarEleccion() {

	nr.mutex.Lock()
	//nr.timeoutTimer.Stop()
	nr.State = Candidate
	nr.votedFor = nr.Yo // Se vota a sí mismo
	nr.mandatoActual++
	peticion := ArgsPeticionVoto{
		CandidateId:  nr.Yo,
		Mandato:      nr.mandatoActual,
		LastLogIndex: nr.lastApplied,
	}
	nr.mutex.Unlock()

	nr.Logger.Printf("Starting election for mandate %d\n", nr.mandatoActual)

	// Lanza una goroutine para cada nodo excepto el propio
	responses := make(chan RespuestaPeticionVoto) // Canal respuestas RPC

	for i := range nr.Nodos {
		if i != nr.Yo {
			go func(nodoID int) {
				var respuesta = RespuestaPeticionVoto{
					VoteGranted: false,
				}
				nr.enviarPeticionVoto(nodoID, &peticion, &respuesta)
				responses <- respuesta // Envía la respuesta recibida al canal
			}(i)
		}
	}

	electionEnd := time.NewTimer(randomElectionTimeout())

	//We voted for ourselves
	grantedVotes := 1
	for {
		select {
		case <-electionEnd.C:
			nr.Logger.Println("**Election ended**")
			nr.mutex.Lock()
			nr.timeoutTimer.Reset(nr.timeoutTime)
			nr.mutex.Unlock()
			nr.Logger.Println("**Election ended 2**")
			return
		case response := <-responses:
			// do stuff. I'd call a function, for clarity:
			if response.VoteGranted {
				grantedVotes++
				if grantedVotes > len(nr.Nodos)/2 {
					close(responses)
					nr.convertirEnLider()
					return
				}
			}
		}
	}
}

func (nr *NodoRaft) convertirEnLider() {
	nr.Logger.Println("I am the leader now")
	nr.mutex.Lock()
	nr.State = Leader
	nr.IdLider = nr.Yo
	nr.Logger.Println("Tocado leader")
	nr.leaderHeartBeatTicker = time.NewTicker(nr.heartbeatTime)
	nr.mutex.Unlock()
	// Inicializar nextIndex y matchIndex para el envío de registros?
}
