# A fresh Debian/Ubuntu developer machine for testing hecatron's Linux
# installers: a non-root user with passwordless sudo and only the packages
# libexec/bootstrap/linux.sh assumes, plus python3 and perl, which hecatron
# itself needs.
ARG BASE=ubuntu:26.04
FROM ${BASE}
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl git gnupg perl python3 sudo unzip xz-utils \
 && rm -rf /var/lib/apt/lists/*
# On PATH for login shells too, which Debian's /etc/profile gives a fixed PATH.
# The checkout is mounted at /hecatron when the container runs.
RUN ln -s /hecatron/bin/hecatron /hecatron/bin/hct /usr/local/bin/
RUN useradd --create-home --shell /bin/bash dev \
 && echo 'dev ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dev
USER dev
WORKDIR /home/dev
