#!/bin/bash


# Función para mostrar el mensaje de uso
mostrar_uso() {
    echo "Uso: $0 [r|t]"
    echo "  r: Despliegue con el estado actual de statefulset (producción)"
    echo "  t: Despliegue con el archivo de pruebas statefulset_go_test.yaml"
    exit 1
}
# Verificar si se pasó un argumento
if [[ $# -ne 1 ]]; then
    echo "Error: Se requiere un solo argumento para indicar el modo de despliegue."
    mostrar_uso
fi
if [[ "$1" != "r" && "$1" != "t" ]]; then
    echo "Error: Argumento inválido '$1'"
    mostrar_uso
    exit 1
fi

# Definir la ruta de la configuración según el modo
if [[ "$1" == "r" ]]; then
    CONFIG_FILE="./Deployment/kube/statefulset_go.yaml"
elif [[ "$1" == "t" ]]; then
    CONFIG_FILE="./Deployment/kube/statefulset_go_test.yaml"
fi

echo "Iniciando clúster kind (si ya existe dará error)"
./Deployment/kube/kind-with-registry.sh
# Variables
SERVER_SRC="./cmd/srvraft"
TESTS_SRC="./internal/testintegracionraft2"
OUTPUT_DIR="./Deployment/bin"
SERVER_BIN="$OUTPUT_DIR/server"
TESTS_BIN_DIR="$OUTPUT_DIR/tests"
CONFIG_CLIENTE="./Deployment/kube/cliente_go.yaml"
# Crear directorios de salida
mkdir -p "$OUTPUT_DIR"
mkdir -p "$TESTS_BIN_DIR"

# Compilación estática del servidor
echo "Compilando servidor de forma estática..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-extldflags "-static"' -o "$SERVER_BIN" "$SERVER_SRC"
if [[ $? -eq 0 ]]; then
    echo "Servidor compilado exitosamente en $SERVER_BIN"
else
    echo "Error al compilar el servidor"
    exit 1
fi

# Compilación estática de los tests
echo "Compilando tests de forma estática..."
for test_file in "$TESTS_SRC"/*_test.go; do
    test_name=$(basename "$test_file" .go)
    test_bin="$TESTS_BIN_DIR/$test_name"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c -o "$test_bin" "$test_file"
    if [[ $? -eq 0 ]]; then
        echo "Test $test_name compilado exitosamente en $test_bin"
    else
        echo "Error al compilar el test $test_name"
        exit 1
    fi
done

echo "Compilación completada. Binarios guardados en $OUTPUT_DIR"
echo "Moviendo binarios a directorios de Dockerfile"
chmod +x ./Deployment/bin/server && mv ./Deployment/bin/server ./Deployment/kube/Dockerfiles/servidor/servidor
chmod +x ./Deployment/bin/tests/testintegracionraft1_test && mv ./Deployment/bin/tests/testintegracionraft1_test ./Deployment/kube/Dockerfiles/cliente/cliente

echo "Generando imágenes y subiéndolas al repositorio"
(
    cd ./Deployment/kube/Dockerfiles/servidor &&
    docker build . -t localhost:5001/servidor:latest &&
    docker push localhost:5001/servidor:latest
)
(
    cd ./Deployment/kube/Dockerfiles/cliente &&
    docker build . -t localhost:5001/cliente:latest &&
    docker push localhost:5001/cliente:latest
)

echo "Creando statefulSet dentro del cluster kind"
kubectl apply -f "$CONFIG_FILE"
echo "Esperando a deployment de servers antes de depslegar el cliente..."
sleep 15
echo "Aplicando deployment del cliente..."
if [[ "$1" == "t" ]]; then
	kubectl apply -f "$CONFIG_CLIENTE"
fi
echo "Esperando a que todo se asiente en el clúster para hacer un get pods..."
sleep 10
kubectl get pods --all-namespaces -o wide
