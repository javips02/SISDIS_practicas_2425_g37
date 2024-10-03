#!/bin/bash

# Variables para las IPs de los workers
MASTER_IP="192.168.3.9"
WORKER_IPS=("192.168.3.10" "192.168.3.11" "192.168.3.12")

# Comando para lanzar el maestro
echo "Iniciando el servidor maestro en $MASTER_IP..."
ssh pi9 "cd /misc/alumnos/sd/sd2425/a815877 && go run cmd/servers/master.go $MASTER_IP:29220" &

# Comando para lanzar workers en la máquina 9 (3 workers)
echo "Iniciando 3 workers en $MASTER_IP..."
ssh pi9 "for port in 29221 29222 29223; do cd /misc/alumnos/sd/sd2425/a815877 && go run cmd/servers/worker.go $MASTER_IP:\$port & done"

# Comandos para lanzar workers en otras máquinas
for WORKER_IP in "${WORKER_IPS[@]}"; do
    echo "Iniciando 4 workers en $WORKER_IP..."
    ssh pi10 "for port in 29221 29222 29223 29224; do cd /misc/alumnos/sd/sd2425/a815877 && go run cmd/servers/worker.go $WORKER_IP:\$port & done"
done

echo "Todos los workers han sido iniciados."
