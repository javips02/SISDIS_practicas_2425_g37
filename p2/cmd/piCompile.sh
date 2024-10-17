echo "Compilando en local para arm..."
GOOS=linux GOARCH=arm64 go build -o actorHendrix actor.go

echo "Subiendo ficheros al servidor"
scp -r actorHendrix a847803@central.cps.unizar.es:/misc/alumnos/sd/sd2425/a847803