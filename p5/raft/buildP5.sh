#!/bin/bash

# Variables
OUTPUT_DIR="./Deployment"
TEST_BINARY_DIR="$OUTPUT_DIR"
MAIN_BINARY_NAME="srvraft"
MAIN_BINARY_PATH="./cmd/srvraft"
GO_VERSION="1.23.3"

# Crear directorios de salida si no existen
mkdir -p "$OUTPUT_DIR"
mkdir -p "$TEST_BINARY_DIR"

# Compilar el ejecutable principal
echo "Compilando el ejecutable principal..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$OUTPUT_DIR/$MAIN_BINARY_NAME" "$MAIN_BINARY_PATH"

if [ $? -ne 0 ]; then
    echo "Error: No se pudo compilar el ejecutable principal."
    exit 1
fi
echo "Ejecutable principal compilado en $OUTPUT_DIR/$MAIN_BINARY_NAME"

# Compilar los tests
echo "Compilando los tests..."
for TEST_FILE in $(find ./internal -name '*_test.go'); do
    TEST_NAME=$(basename "$TEST_FILE" .go)
    TEST_DIR=$(dirname "$TEST_FILE")
    OUTPUT_TEST_BINARY="$TEST_BINARY_DIR/$TEST_NAME"

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o "$OUTPUT_TEST_BINARY" "$TEST_DIR"

    if [ $? -ne 0 ]; then
        echo "Error: No se pudo compilar el test $TEST_FILE."
        exit 1
    fi
    echo "Test compilado: $OUTPUT_TEST_BINARY"
done

chmod +x ./Deployment/srvraft
chmod +x ./Deployment/testintegracionraft1_test

mv ./Deployment/srvraft ./Deployment/kube/Dockerfiles/servidor/servidor
mv ./Deployment/testintegracionraft1_test ./Deployment/kube/Dockerfiles/cliente/cliente

echo "Todos los ejecutables y tests han sido compilados con éxito."

echo "COMENZANDO A DOCKERIZAR EL PROYECTO"
cd ./Deployment/kube/Dockerfiles/servidor && docker build . -t localhost:5000/servidor:latest && docker push localhost:5000/servidor:latest 
cd ./Deployment/kube/Dockerfiles/cliente && docker build . -t localhost:5000/cliente:latest && docker push localhost:5000/cliente:latest

