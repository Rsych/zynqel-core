FROM node:22-bookworm-slim

# Common dev tools
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        curl \
        wget \
        ca-certificates \
        jq \
        procps \
        openssh-client \
        build-essential \
        python3 \
        python3-pip \
        vim \
        less \
        htop \
        unzip \
        tree \
        ripgrep \
        locales \
    && rm -rf /var/lib/apt/lists/* \
    && sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen

# UTF-8 locale — required for box-drawing and Unicode in terminal UIs
ENV LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

# Shell prompt
RUN echo 'PS1="\[\e[32m\]\w\[\e[0m\] \$ "' >> /root/.bashrc && \
    echo 'alias ls="ls --color=auto"' >> /root/.bashrc && \
    echo 'alias ll="ls -la"' >> /root/.bashrc

# Create workspace directory
RUN mkdir -p /workspace
WORKDIR /workspace

SHELL ["/bin/bash", "-c"]
CMD ["sleep", "infinity"]
