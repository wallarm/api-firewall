ARG DOCKER_VERSION
ARG KIND_VERSION
ARG KUBERNETES_VERSION

FROM docker:${DOCKER_VERSION}-dind as builder

ARG KIND_VERSION

WORKDIR /output
COPY ./manifests ./manifests
COPY ./scripts ./usr/local/bin

RUN chmod +x ./usr/local/bin/*

RUN apk add --no-cache                                 \
        go                                             \
        git                                            \
        git-lfs                                     && \
    export GO111MODULE="on"                         && \
    go get sigs.k8s.io/kind@v${KIND_VERSION}        && \
    mkdir -p ./usr/local/bin                        && \
    cp /root/go/bin/kind ./usr/local/bin/kind

FROM docker:${DOCKER_VERSION}-dind

ARG KUBERNETES_VERSION
ARG HELM_VERSION_2=2.17.0
ARG HELM_VERSION_3=3.6.0

USER root

RUN apk add --no-cache                                      \
        bash                                                \
        bash-completion                                     \
        bind-tools                                          \
        curl                                                \
        findutils                                           \
        gettext                                             \
        jq                                                  \
        nano                                                \
        py3-pip                                             \
        python3                                             \
        shadow                                              \
        supervisor                                       && \
    mkdir -p /etc/bash_completion.d                      && \
    curl https://storage.googleapis.com/kubernetes-release/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubectl \
        -Lo /usr/local/bin/kubectl                       && \
    chmod +x /usr/local/bin/kubectl                      && \
    kubectl completion bash                                 \
        >> /etc/bash_completion.d/kubectl.bash           && \
    curl helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION_3}-linux-amd64.tar.gz \
        | tar xz --strip-components=1 linux-amd64/helm   && \
    mv helm /usr/local/bin/helm3                         && \
    chmod +x /usr/local/bin/helm3                        && \
    curl helm.tar.gz https://get.helm.sh/helm-v${HELM_VERSION_2}-linux-amd64.tar.gz \
        | tar xz --strip-components=1 linux-amd64/helm   && \
    mv helm /usr/local/bin/helm2                         && \
    chmod +x /usr/local/bin/helm2                        && \
    ln -sf /usr/local/bin/helm3 /usr/local/bin/helm      && \
    helm completion bash                                    \
        >> /etc/bash_completion.d/helm.bash              && \
    pip install --no-cache --no-cache-dir                   \
        yq

COPY --from=builder /output /
