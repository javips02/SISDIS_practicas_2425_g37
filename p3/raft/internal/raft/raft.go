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
	Operation string // La operaciones posibles son "leer" y "escribir"
	Key       string
	Value     string // en el caso de la lectura Valor = ""
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
	mutex              sync.Mutex // Mutex para proteger acceso a estado compartido
	barreraDistribuida sync.Mutex //Mutex para barrera distribuida
	// Host:Port de todos los nodos (réplicas) Raft, en mismo orden
	Nodos    []rpctimeout.HostPort
	Yo       int // indice de este nodos en campo array "nodos"
	LeaderId int
	State    State

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

	currentTerm int // Indica el mandato más reciente que esta réplica conoce
	votedFor    int //candidato que ha recibido el voto en el mandato actual
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
func NuevoNodo(nodos []rpctimeout.HostPort, yo int, shouldWait int,
	canalAplicarOperacion chan AplicaOperacion) *NodoRaft {
	nr := &NodoRaft{}
	nr.Nodos = nodos
	nr.Yo = yo
	nr.currentTerm = 0
	nr.LeaderId = -1
	nr.State = Follower

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

	if shouldWait == 1 {
		nr.Logger.Println("Blocking barrera distribuida")
		nr.barreraDistribuida.Lock()
	}

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
	esLider := nr.LeaderId == nr.Yo
	idLider := nr.LeaderId
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
	mandato := nr.currentTerm
	EsLider := nr.LeaderId == nr.Yo
	idLider := nr.LeaderId
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
		Term:        nr.currentTerm,
		LeaderId:    nr.Yo,
		PrevLogTerm: 0,

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
	successFlag := false
	nr.mutex.Lock()
	//para esta practica el objetivo es comprometer en todos los nodos
	if successful == len(nr.Nodos) {
		successFlag = true
		indice = nr.commitIndex
		mandato = nr.currentTerm
		EsLider = nr.LeaderId == nr.Yo
		idLider = nr.LeaderId
		valorADevolver = "" //Don't touch it for now
	} else {
		successFlag = true
		indice = nr.commitIndex
		mandato = nr.currentTerm
		EsLider = nr.LeaderId == nr.Yo
		idLider = nr.LeaderId
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
	nr.Logger.Println(nr.obtenerEstado())
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
	Mandato      int //(para pr4)
	CandidateId  int //candidato pidiendo el voto
	LastLogIndex int // indice de la ultima entrada del log del candidato
	//lastLogTerm int (para pr4)
}

// Recordar
// -----------
// Nombres de campos deben comenzar con letra mayuscula !
type RespuestaPeticionVoto struct {
	// Vuestros datos aqui
	VoteGranted bool
	Mandate     int
}

// Metodo para RPC PedirVoto
func (nr *NodoRaft) PedirVoto(peticion *ArgsPeticionVoto,
	reply *RespuestaPeticionVoto) error {

	nr.mutex.Lock()

	//A process remains a follower as long as he gets RPCs from leaders or candidates
	nr.timeoutTimer.Reset(nr.timeoutTime)
	if peticion.Mandato > nr.currentTerm {
		nr.currentTerm = peticion.Mandato
		nr.votedFor = -1
		nr.State = Follower
	}

	nr.Logger.Print("Mandate ", peticion.Mandato, "got PedirVoto by ", peticion.CandidateId)
	if nr.votedFor == -1 {
		nr.votedFor = peticion.CandidateId
		reply.VoteGranted = true
	} else {
		reply.VoteGranted = false
	}
	reply.Mandate = nr.currentTerm
	nr.mutex.Unlock()

	if reply.VoteGranted {
		nr.Logger.Print("Granted vote request to ", peticion.CandidateId)
	} else {
		nr.Logger.Print("Denied vote request to ", peticion.CandidateId)
	}
	nr.Logger.Println("Mandate ", peticion.Mandato, " voted for: ", nr.votedFor)

	//TODO: Devolver mandato?

	return nil
}

type ArgsAppendEntries struct {
	Term        int //Leader term
	LeaderId    int
	PrevLogTerm int //term of prevLogIndex entry

	Entries      []Entry
	LeaderCommit int // eader’s commitIndex

}

type ReplyAppendEntries struct {
	Term    int
	Success bool
	Node    int
}

type Results struct {
	Success bool
}

// El metodo que el leader llama en los seguidores para insertar una nueva entrada en los seguidores
// Pueden insertarse varias entradas de un paso, por ejemplo cuando el nodo revive despues de un fallo :)
func (nr *NodoRaft) StartNode(args *Vacio,
	results *Vacio) error {

	nr.barreraDistribuida.Unlock()
	return nil //si llega hasta aquí, return NoError (error nil) de RPC
}

// El metodo que el leader llama en los seguidores para insertar una nueva entrada en los seguidores
// Pueden insertarse varias entradas de un paso, por ejemplo cuando el nodo revive despues de un fallo :)
func (nr *NodoRaft) AppendEntries(args *ArgsAppendEntries,
	results *Results) error {

	nr.mutex.Lock()
	nr.timeoutTimer.Reset(nr.timeoutTime)

	if nr.currentTerm <= args.Term {
		nr.convertirEnFollower(args.Term, args.LeaderId)
	}
	nr.mutex.Unlock()
	if len(args.Entries) == 0 {
		results.Success = true
		return nil
	}

	results.Success = true //sin mandatos siempre será success
	nr.mutex.Lock()
	for _, value := range args.Entries {
		nr.lastApplied++
		nr.Logs[nr.lastApplied] = value //meter el comando con su índice
		nr.Logger.Println("Entrada anyadida: ", nr.Logs[nr.lastApplied])
		nr.Logger.Println("Log entero: ", nr.Logs)
	}
	nr.mutex.Unlock()

	// if leader commit > commit index form current node, choose min
	/*if args.LeaderCommit > nr.commitIndex {
		nr.commitIndex = min(args.LeaderCommit, nr.commitIndex)
	}*/
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
	//We wait for the test client to unlock the nodes
	nr.barreraDistribuida.Lock()
	nr.timeoutTimer = time.NewTimer(nr.timeoutTime)
	nr.leaderHeartBeatTicker = time.NewTicker(nr.heartbeatTime)
	nr.leaderHeartBeatTicker.Stop()
	for {
		select {
		//In this case, leader hasn't sent a heartbeat in a while, so we start eection
		case <-nr.timeoutTimer.C: // Election timeout case
			nr.Logger.Println("Election!")
			nr.iniciarEleccion() // Start election

		case <-nr.leaderHeartBeatTicker.C: // Leader heartbeat case
			go nr.enviarLatidosATodos()
		}
	}
}

func (nr *NodoRaft) enviarLatidosATodos() {
	args := ArgsAppendEntries{
		Term:         nr.currentTerm,
		LeaderId:     nr.Yo,
		PrevLogTerm:  0,
		Entries:      make([]Entry, 0),
		LeaderCommit: nr.commitIndex,
	}
	for i := 0; i < len(nr.Nodos); i++ {
		go func(nr *NodoRaft, nodo int, args *ArgsAppendEntries) {
			reply := Vacio{}
			if nodo != nr.Yo {
				err := nr.Nodos[nodo].CallTimeout("NodoRaft.AppendEntries", args, &reply, 10*time.Millisecond)
				if err != nil {
					nr.Logger.Println("Failed to send beat to ", nodo)
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
	nr.currentTerm++
	peticion := ArgsPeticionVoto{
		CandidateId:  nr.Yo,
		Mandato:      nr.currentTerm,
		LastLogIndex: nr.lastApplied,
	}
	nr.mutex.Unlock()

	nr.Logger.Printf("Starting election for mandate %d\n", nr.currentTerm)

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
			nr.mutex.Lock()
			nr.timeoutTimer.Reset(nr.timeoutTime)
			nr.Logger.Println("**Election ended for mandate **", nr.currentTerm)
			nr.mutex.Unlock()
			return
		case response := <-responses:
			// do stuff. I'd call a function, for clarity:
			nr.mutex.Lock()
			if response.VoteGranted && response.Mandate == nr.currentTerm {
				nr.mutex.Unlock()
				grantedVotes++
				if grantedVotes > len(nr.Nodos)/2 {
					nr.convertirEnLeader()
					return
				}
			} else {
				nr.mutex.Unlock()
			}
			//TODO:
			//case si recibo pedirElecion con mandato mas alto?
		}
	}
}

func (nr *NodoRaft) convertirEnLeader() {
	nr.Logger.Println("I am the leader now")
	nr.mutex.Lock()
	nr.State = Leader
	nr.LeaderId = nr.Yo
	nr.leaderHeartBeatTicker.Reset(nr.heartbeatTime)
	nr.mutex.Unlock()
	// Inicializar nextIndex y matchIndex para el envío de registros?
}

// TO BE CALLED ONLY WITH MUTEX CONTROL
func (nr *NodoRaft) convertirEnFollower(mandate int, leader int) {
	//DON'T LOCK, FUNCTION MUST BE CALLED WITH MUTEX CONTROL
	nr.State = Follower
	nr.LeaderId = leader
	nr.currentTerm = mandate
	nr.leaderHeartBeatTicker.Stop()
}
