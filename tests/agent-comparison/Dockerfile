# The machine each agent starts on: a fresh Ubuntu 24.04 with sudo, network
# and Claude Code, and no language toolchains. Python 3 and perl are there
# in both arms because hecatron's installer adds them, so that the arms
# differ only in hecatron (its latest release, as a newcomer installs it)
# and its Agent Skill.
FROM ubuntu:24.04 AS plain
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl git sudo xz-utils unzip python3 perl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -m -s /bin/bash dev \
    && echo 'dev ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/dev
USER dev
WORKDIR /home/dev
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH=/home/dev/.local/bin:${PATH}

FROM plain AS hecatron
# The release run.sh found; a new one rebuilds this layer.
ARG HECATRON_RELEASE
RUN curl -fsSL https://raw.githubusercontent.com/dnf/hecatron/main/install.sh | sh \
    && bash -lc 'hct --version && hct skill install'
