FROM docker.gitea.com/gitea:1.24.6


# Copy initial database
COPY gitea.db /data/gitea/gitea.db

# Prepare repository and initialize bare repo
RUN mkdir -p /data/git/repositories/onoffinc && \
  chown -R git:git /data/git/repositories && \
  su - git -c "git init -b main --bare /data/git/repositories/onoffinc/features.git" && \
  git config --global --add safe.directory /data/git/repositories/onoffinc/features.git

# Clone, add initial file, commit, push, cleanup
WORKDIR /tmp/repo
RUN git clone /data/git/repositories/onoffinc/features.git . && \
  mkdir -p default && \
  mkdir -p admin && \
  git config --global user.email 'admin@example.com' && \
  git config --global user.name 'admin'

COPY default-features.yaml default/features.yaml
COPY admin-features.yaml admin/features.yaml
COPY workflows/lint.yaml .gitea/workflows/lint.yaml

RUN git add default admin .gitea && \
  git commit -m "Initial commit" && \
  git push origin main && \
  cd / && rm -rf /tmp/repo

# Copy hooks and fix permissions
COPY hooks/ /data/git/repositories/onoffinc/features.git/hooks/
RUN chown -R git:git /data/git/repositories

WORKDIR /
