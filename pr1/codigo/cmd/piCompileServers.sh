echo "Compilando en local para arm..."
GOOS=linux GOARCH=arm64 go build -o bin/ ./client/clientCargaMaxima.go
GOOS=linux GOARCH=arm64 go build -o bin/serverSecuencial ./server-draft/example.go
GOOS=linux GOARCH=arm64 go build -o bin/ ./servers/infiniteGoRoutinesServer.go
GOOS=linux GOARCH=arm64 go build -o bin/ ./servers/goRoutinesPoolServer.go
GOOS=linux GOARCH=arm64 go build -o bin/ ./servers/master.go
GOOS=linux GOARCH=arm64 go build -o bin/ ./servers/worker.go

echo "Subiendo ficheros al servidor"
scp -r ./bin/* a847803@central.cps.unizar.es:/misc/alumnos/sd/sd2425/a847803