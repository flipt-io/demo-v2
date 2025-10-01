FROM docker.gitea.com/gitea:1.24.6

COPY gitea.db /data/gitea/gitea.db
RUN mkdir -p /data/git/repositories/onoffinc 
RUN chown -R git:git /data/git/repositories/onoffinc
RUN su - git -c 'git init --bare /data/git/repositories/onoffinc/features.git'
