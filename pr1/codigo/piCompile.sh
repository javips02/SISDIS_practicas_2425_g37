echo "Compilando en local para arm..."
GOOS=linux GOARCH=arm64 go build -o bin/ ./...

echo "Subiendo ficheros al servidor"
scp -r ./bin/* a815877@central.cps.unizar.es:/misc/alumnos/sd/sd2425/a815877