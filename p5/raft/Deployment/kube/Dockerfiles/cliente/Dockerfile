FROM alpine:latest

# Instalar cliente SSH
RUN apk update && \
    apk add --no-cache openssh

# Copia claves para root
RUN mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh
COPY ssh_key.pub /root/.ssh/authorized_keys
COPY ssh_key /root/.ssh/id_rsa
RUN chmod 600 /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/id_rsa

# Configuración para evitar prompts de confirmación
RUN echo "Host *.ss-service.default.svc.cluster.local\n\
    StrictHostKeyChecking no\n\
    UserKnownHostsFile /dev/null" > /root/.ssh/config

# Copiar el script del cliente (opcional)
COPY cliente /cliente
RUN chmod +x /cliente

EXPOSE 6000 6001 22
CMD ["/bin/sh"]
