FROM alpine:latest

# Instalar cliente SSH
RUN apk update && \
    apk add --no-cache openssh

# Copiar las claves privadas para autenticación
COPY ssh_key /root/.ssh/id_rsa
RUN chmod 600 /root/.ssh/id_rsa && \
    mkdir -p /root/.ssh

# Configuración para evitar prompts de confirmación
RUN echo "Host *.ss-service.default.svc.cluster.local\n\
    StrictHostKeyChecking no\n\
    UserKnownHostsFile /dev/null" > /root/.ssh/config

# Copiar el script del cliente (opcional)
COPY cliente /cliente
RUN chmod +x /cliente

CMD ["/bin/sh"]
