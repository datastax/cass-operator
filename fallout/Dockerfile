### Stage 1: Set up base python venv
FROM python:3.7.4-alpine3.10 as base_pyenv

RUN apk add openssh git bash bind-tools && python3 -m venv fallout-cli-venv

### Stage 2: Install fallout-cli
FROM base_pyenv as private_fallout_git

ARG GITHUB_TOKEN

RUN . /fallout-cli-venv/bin/activate && \
      pip install --upgrade "git+https://${GITHUB_TOKEN}@github.com/riptano/fallout.git@dbfea40ab988d15059e95a9c3498795ce6be915a#egg=fallout-client&subdirectory=fallout-cli"

### Stage 3: copy pyvenv with fallout-cli installed
FROM base_pyenv as final

# We have to be careful when copying around virtual envs,
# but since we have the same path in the source and destination
# we are safe in this case
COPY --from=private_fallout_git /fallout-cli-venv /fallout-cli-venv

ENV VIRTUAL_ENV "/fallout-cli-venv"
ENV PATH "$VIRTUAL_ENV/bin:$PATH"

WORKDIR /build

CMD "/bin/bash"
