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
	"os"

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

type Entry struct {
	Operacion string // La operaciones posibles son "leer" y "escribir"
	Clave     string
	Valor     string // en el caso de la lectura Valor = ""
	Mandato   int    //mandato al cual pertenece esta entrada
}

// A medida que el nodo Raft conoce las operaciones de las  entradas de registro
// comprometidas, envía un AplicaOperacion, con cada una de ellas, al canal
// "canalAplicar" (funcion NuevoNodo) de la maquina de estados
type AplicaOperacion struct {
	Indice    int // en la entrada de registro
	Operacion Entry
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
	Logger *log.Logger
	Logs   []Entry
	// Vuestros datos aqui.

	// VALORES PERSISTENTES EN TODOS LOS SERVIDORES //

	mandatoActual int // Indica el mandato más reciente que esta réplica conoce
	votedFor      int //candidato que ha recibido el voto en el mandato actual
	//numVotes      int //number of votes recieved in the current election
	//el log es Entries map[string][string] que venía dado

	// VALORES VOLÁTILES DE ESTADO EN TODOS LOS SERVIDORES //

	commitIndex  int         // index of highest log entry known to be committed
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
	//barrera.Add(1)

	if kEnableDebugLogs {
		nombreNodo := nodos[yo].Host() + "_" + nodos[yo].Port()
		log.Println("nombreNodo: ", nombreNodo)

		if kLogToStdout {
			nr.Logger = log.New(os.Stdout, nombreNodo+" -->> ",
				log.Lmicroseconds|log.Lshortfile)
		} else {
			err := os.MkdirAll(kLogOutputDir, os.ModePerm)
			if err != nil {
				nr.Logger.Println(err)
				panic(err)
			}
			logOutputFile, err := os.OpenFile(
				fmt.Sprintf("%s/%s.txt", kLogOutputDir, nombreNodo),
				os.O_RDWR|os.O_CREATE|os.O_TRUNC,
				0755)
			if err != nil {
				nr.Logger.Println(err)
				panic(err)
			}
			nr.Logger = log.New(logOutputFile,
				nombreNodo+" -> ", log.Lmicroseconds|log.Lshortfile)
		}
		file, err := os.Create(fmt.Sprint("output_", nr.Yo, ".txt"))

		if err != nil {
			nr.Logger.Println(err)
		}
		defer file.Close()

		// Redirect stdout to the file
		_ = os.Stdout
		os.Stdout = file
		os.Stderr = file

		nr.Logger.Println("logger initialized")
	} else {
		nr.Logger = log.New(io.Discard, "", 0)
	}

	// Añadir codigo de inicialización
	// Inicialización de otros campos
	nr.Logs = make([]Entry, 100) //
	nr.votedFor = -1             // No ha votado aún
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

	nr.timeoutTime = randomElectionTimeout()
	nr.heartbeatTime = heartbeatTimeout()

	nr.Logger.Println("Election timeout: ", nr.timeoutTime, "ms")
	nr.Logger.Println("Heartbeat interval: ", nr.heartbeatTime, "ms")

	nr.timeoutTimer = time.NewTimer(nr.timeoutTime)
	nr.leaderHeartBeatTicker = time.NewTicker(nr.heartbeatTime)
	nr.leaderHeartBeatTicker.Stop()

	go nr.monitorizarTemporizadoresRaft() // monitorizar timeout eleccion y HB
	//IdNodo, Mandato, EsLider, IdLider := nr.obtenerEstado()
	//nr.Logger.Println(nr.obtenerEstado())
	return nr
}

// Metodo randomElectionTimeout() utilizado cuando queremos asignar un tiempo
// limite sin lider dentor de cada nodo. Devuelve un tiempo aleatorio en ms
// entre 200 y 400
func randomElectionTimeout() time.Duration {
	return time.Duration(800+rand.Intn(1500)) * time.Millisecond
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
	mandato := nr.mandatoActual

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
// - La operacion si consigue comprometerse.
// - El indice del registro donde se va a colocar
// - El mandato en curso
// - true si el nodo cree ser el lider
// - El indice del líder
// - El resultado de aplicar esta operación en máquina de estados
func (nr *NodoRaft) someterOperacion(operacion Entry) (bool, int, int,
	bool, int, string) {

	nr.mutex.Lock()
	indice := nr.commitIndex
	mandato := nr.mandatoActual
	EsLider := nr.IdLider == nr.Yo
	idLider := nr.IdLider
	nr.mutex.Unlock()

	valorADevolver := ""

	// no lider => devolver falso (incluye quién es lider en la respuesta, el cliente tiene que reenviar
	if !EsLider {
		valorADevolver = "false"
		return false, indice, mandato, EsLider, idLider, valorADevolver
	}

	nr.Logger.Println(operacion)
	// Definir la operación a someter

	//var entries []Entry
	entries := make([]Entry, 1)
	entries[0] = operacion
	nr.mutex.Lock()
	args := ArgsAppendEntries{
		Term:        nr.mandatoActual,
		LeaderId:    nr.Yo,
		prevLogTerm: 0,

		Entries:      entries,
		LeaderCommit: nr.commitIndex,
	}
	nr.mutex.Unlock()
	repliesChan := make(chan ReplyAppendEntries, len(nr.Nodos))

	for i := range nr.Nodos {
		if i != nr.Yo {
			go func(nodo int, args ArgsAppendEntries, repliesChan chan ReplyAppendEntries) {
				peer := nr.Nodos[nodo]
				reply := ReplyAppendEntries{
					Node:    nodo,
					Success: false,
				}
				nr.Logger.Println("Valor de args: ", args)
				err := peer.CallTimeout("NodoRaft.AppendEntries", &args, &reply, 50*time.Millisecond) // TODO: ajustar timeout
				nr.Logger.Printf("Enviada AppendEntries a %d, succeso? ", nodo)
				if reply.Success {
					nr.Logger.Printf("SI!\n")
				} else {
					nr.Logger.Printf("NO!\n")
				}
				if err != nil {
					repliesChan <- ReplyAppendEntries{
						Node:    nodo,
						Success: false,
					}
				} else {
					repliesChan <- reply
				}
			}(i, args, repliesChan)
		}
	}

	nr.Logger.Println("Starting to read replies")
	successful := 0
	for i := 0; i < len(nr.Nodos)-1; i++ {
		nr.Logger.Println("i: ", i)
		reply := <-repliesChan
		if reply.Success {
			successful++
		}
	}
	nr.Logger.Println()
	successFlag := false
	nr.mutex.Lock()
	nr.Logger.Println()
	//para esta practica el objetivo es comprometer en todos los nodos
	if successful == len(nr.Nodos) {
		successFlag = true
		indice = nr.commitIndex
		mandato = nr.mandatoActual
		EsLider = nr.IdLider == nr.Yo
		idLider = nr.IdLider
		valorADevolver = "" //Don't touch it for now
	} else {
		successFlag = true
		indice = nr.commitIndex
		mandato = nr.mandatoActual
		EsLider = nr.IdLider == nr.Yo
		idLider = nr.IdLider
		valorADevolver = "" //Don't touch it for now
	}

	nr.mutex.Unlock()
	return successFlag, indice, mandato, EsLider, idLider, valorADevolver
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
	//nr.Logger.Println(nr.obtenerEstado())
	return nil
}

type ResultadoRemoto struct {
	Success        bool
	ValorADevolver string
	IndiceRegistro int
	EstadoParcial
}

func (nr *NodoRaft) SometerOperacionRaft(operacion *Entry,
	reply *ResultadoRemoto) error {
	reply.Success, reply.IndiceRegistro, reply.Mandato, reply.EsLider,
		reply.IdLider, reply.ValorADevolver = nr.someterOperacion(*operacion)
	return nil
}

// -----------------------------------------------------------------------
// LLAMADAS RPC protocolo RAFT
//
// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type ArgsPeticionVoto struct {
	// Vuestros datos aqui
	Term         int //(para pr4)
	CandidateId  int //candidato pidiendo el voto
	LastLogIndex int // indice de la ultima entrada del log del candidato
	LastLogTerm  int //(para pr4)
}

// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type RespuestaPeticionVoto struct {
	// Vuestros datos aqui
	VoteGranted bool
	Mandate     int
}

// Reset heartbeat counter
func (nr *NodoRaft) Heartbeat(args *HeartbeatArgs,
	output *Vacio) error {
	//nr.Logger.Println("Latido desde ", args.IdLeader, " mandato ", args.Mandato)
	nr.timeoutTimer.Reset(nr.timeoutTime)
	nr.mutex.Lock()
	if nr.mandatoActual < args.Mandato {
		nr.convertirEnFollower(args.Mandato, args.IdLeader)
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
	if peticion.Term > nr.mandatoActual {
		nr.mandatoActual = peticion.Term
		nr.votedFor = -1
		nr.State = Follower
	}

	nr.Logger.Print("Mandate ", peticion.Term, "got PedirVoto by ", peticion.CandidateId)
	if nr.votedFor == -1 {
		nr.votedFor = peticion.CandidateId
		reply.VoteGranted = true
	} else {
		reply.VoteGranted = false
	}
	reply.Mandate = nr.mandatoActual
	nr.mutex.Unlock()

	if reply.VoteGranted {
		nr.Logger.Print("Granted vote request to ", peticion.CandidateId)
	} else {
		nr.Logger.Print("Denied vote request to ", peticion.CandidateId)
	}
	nr.Logger.Println("Mandate ", peticion.Term, " voted for: ", nr.votedFor)

	return nil
}

type ArgsAppendEntries struct {
	Term        int //Leader term
	LeaderId    int
	prevLogTerm int //term of prevLogIndex entry

	Entries      []Entry
	LeaderCommit int // eader’s commitIndex

}

type ReplyAppendEntries struct {
	Term    int
	Success bool
	Node    int
}

type HeartbeatArgs struct {
	IdLeader int
	Mandato  int
}

type Results struct {
	Success bool
	Term    int
}

// El metodo RPC que el leader llama en los seguidores para insertar una nueva
// entrada en los seguidores.
// Pueden insertarse varias entradas de un paso, por ejemplo cuando el nodo
// revive despues de un fallo :)
func (nr *NodoRaft) AppendEntries(args *ArgsAppendEntries,
	results *Results) error {
	results.Success = true //sin mandatos siempre será success
	// 1. if term < currentTerm --> reply false and the term for the current node
	if args.Term < nr.mandatoActual {
		results.Term = nr.mandatoActual
		results.Success = false
		return nil
	}
	// 2. if !exists entry at prevLogIndex == term from prevLogTerm --> reply false
	if len(nr.Logs) < args.prevLogTerm ||
		nr.Logs[args.prevLogTerm].Mandato != args.prevLogTerm {
		results.Term = nr.mandatoActual
		results.Success = false
		return nil
	}
	//3. if newEntry.index == otherEntry.index && termNew != termOther --> reply false
	for i, entry := range nr.Logs {
		fmt.Printf("Operacion: %s, Clave: %s, Valor: %s, Mandato: %d\n",
			entry.Operacion, entry.Clave, entry.Valor, entry.Mandato)
		if args.LeaderCommit == i && args.Term != nr.Logs[i].Mandato {
			nr.mutex.Lock()
			nr.Logs = nr.Logs[:i]
			nr.mutex.Unlock()
		}
	}

	//4. Append any new entries not already in the log
	nr.mutex.Lock()
	nr.Logs = append(nr.Logs[:nr.lastApplied+1], args.Entries...)
	nr.lastApplied = len(nr.Logs) - 1
	nr.Logger.Println("Log actualizado: ", nr.Logs)
	nr.mutex.Unlock()

	//5. If leader commit > commit index form current node,
	//	choose min(leaderCommit, index of last new entry)
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

	//nr.Logger.Println(nodo, args, reply)
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
	for {
		select {
		case <-nr.timeoutTimer.C:
			nr.mutex.Lock()
			if nr.State != Leader {
				nr.iniciarEleccion()
			}
			nr.mutex.Unlock()
		case <-nr.leaderHeartBeatTicker.C:
			if nr.State == Leader {
				nr.enviarLatidosATodos()
			}
		}
	}
}

func (nr *NodoRaft) iniciarEleccion() {

	nr.mutex.Lock()
	//nr.timeoutTimer.Stop()
	nr.State = Candidate
	nr.mandatoActual++
	nr.votedFor = nr.Yo // Se vota a sí mismo
	nr.timeoutTimer.Reset(randomElectionTimeout())

	peticion := ArgsPeticionVoto{
		Term:         nr.mandatoActual,
		CandidateId:  nr.Yo,
		LastLogIndex: nr.lastApplied,
		LastLogTerm:  nr.Logs[nr.lastApplied].Mandato,
	}
	nr.mutex.Unlock()

	nr.Logger.Printf("Starting election for mandate %d\n", nr.mandatoActual)

	// Canal respuestas RPC bufferizado (soporte a respuestas concurrentes)
	responses := make(chan RespuestaPeticionVoto, len(nr.Nodos))

	// Lanza una goroutine para cada nodo excepto el propio
	for i := range nr.Nodos {
		if i != nr.Yo {
			go func(nodoID int) {
				var respuesta = RespuestaPeticionVoto{
					VoteGranted: false,
				}
				nr.enviarPeticionVoto(nodoID, &peticion, &respuesta)
				responses <- respuesta // Envía la respuesta recibida al canal
				nr.Logger.Printf(
					"Node %d responded with VoteGranted=%v for "+
						"mandate %d\n", nodoID, respuesta.VoteGranted,
					respuesta.Mandate)
			}(i)
		}
	}

	electionEnd := time.NewTimer(randomElectionTimeout())

	//We voted for ourselves
	grantedVotes := 1
	for {
		select {
		case <-electionEnd.C:
			nr.Logger.Println("Election timeout. Returning to follower state.")
			nr.mutex.Lock()
			nr.State = Follower
			nr.votedFor = -1
			nr.mutex.Unlock()
			return
		case response := <-responses:
			nr.mutex.Lock()
			if response.Mandate > nr.mandatoActual {
				nr.Logger.Println("Found higher mandate. Returning to follower state.")
				nr.mandatoActual = response.Mandate
				nr.State = Follower
				nr.votedFor = -1
				nr.mutex.Unlock()
				return
			}
			if response.VoteGranted && response.Mandate == nr.mandatoActual {
				grantedVotes++
				nr.Logger.Printf("Vote granted! Total: %d\n", grantedVotes)
				if grantedVotes > len(nr.Nodos)/2 {
					nr.convertirEnLeader()
					nr.mutex.Unlock()
					return
				}
			}
			nr.mutex.Unlock()
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
				err := nr.Nodos[nodo].CallTimeout(
					"NodoRaft.Heartbeat",
					args,
					&reply,
					10*time.Millisecond)
				if err != nil {
					nr.Logger.Println(
						"Failed to send beat to ", nodo)
				}
			}
		}(nr, i, &args)
	}
}

func (nr *NodoRaft) convertirEnLeader() {
	nr.Logger.Println("I am the leader now")
	nr.mutex.Lock()
	nr.Logger.Println("Toy akì")
	nr.State = Leader
	nr.IdLider = nr.Yo
	nr.leaderHeartBeatTicker.Reset(nr.heartbeatTime)
	nr.Logger.Println("IdLeader: ", nr.IdLider)
	nr.mutex.Unlock()
	// Inicializar nextIndex y matchIndex para el envío de registros?
}

// TO BE CALLED ONLY WITH MUTEX CONTROL
func (nr *NodoRaft) convertirEnFollower(mandate int, leader int) {
	//DON'T LOCK, FUNCTION MUST BE CALLED WITH MUTEX CONTROL
	nr.State = Follower
	nr.IdLider = leader
	nr.mandatoActual = mandate
	nr.leaderHeartBeatTicker.Stop()
}
