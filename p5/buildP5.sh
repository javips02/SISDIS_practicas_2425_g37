#!/bin/bash

# Parar en caso de errores
set -e

# Rutas
SRC_DIR="../p4/raft"
BUILD_DIR="./bin"
MAIN_FILE="$SRC_DIR/cmd/srv/main.go"
TESTS_DIR="$SRC_DIR"

# Crear el directorio de salida si no existe
mkdir -p "$BUILD_DIR"

# Compilar el programa principal de manera estática
echo "Compilando el binario principal de forma estática desde $SRC_DIR..."
CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o "$BUILD_DIR/raft_server" "$MAIN_FILE"

echo "Binario principal generado: $BUILD_DIR/raft_server"

# Ejecutar pruebas del proyecto
echo "Ejecutando pruebas desde $TESTS_DIR..."
pushd "$TESTS_DIR" > /dev/null
go test ./...
popd > /dev/null

echo "Pruebas ejecutadas con éxito."

# Finalizar
echo "Compilación completada. Todos los binarios y pruebas están listos."
