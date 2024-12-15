FROM alpine:latest
RUN apk update && \
    apk add --no-cache openssh openssl && \
    mkdir -p .ssh

# Copia claves para root
RUN mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh
COPY ssh_key.pub /root/.ssh/authorized_keys
COPY ssh_key /root/.ssh/id_rsa
RUN chmod 600 /root/.ssh/authorized_keys && \
    chmod 600 /root/.ssh/id_rsa

# Configuración SSH
RUN sed -i 's/#PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PermitRootLogin.*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config

# Generar claves de host para SSH
RUN ssh-keygen -A

COPY servidor /servidor
EXPOSE 6000 6001 22

# Comando de entrada para iniciar SSH y tu servidor
CMD ["/bin/sh", "-c", "/usr/sbin/sshd -D & /servidor"]
