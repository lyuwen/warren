# Docker SSH Server for Warren Integration Testing
FROM ubuntu:22.04

# Install SSH server, tmux, and basic tools
RUN apt-get update && \
    apt-get install -y \
    openssh-server \
    tmux \
    curl \
    vim \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Create SSH directory
RUN mkdir /var/run/sshd

# Create test user
RUN useradd -m -s /bin/bash testuser && \
    echo 'testuser:testpass' | chpasswd && \
    usermod -aG sudo testuser

# Configure SSH
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# Create .ssh directory for testuser
RUN mkdir -p /home/testuser/.ssh && \
    chmod 700 /home/testuser/.ssh && \
    chown testuser:testuser /home/testuser/.ssh

# Create tmux config for testuser
RUN echo "set -g default-terminal 'screen-256color'" > /home/testuser/.tmux.conf && \
    echo "set -g history-limit 10000" >> /home/testuser/.tmux.conf && \
    chown testuser:testuser /home/testuser/.tmux.conf

# Expose SSH port
EXPOSE 22

# Start SSH service
CMD ["/usr/sbin/sshd", "-D"]
