# syntax = docker/dockerfile:1.2
ARG BASE_DOCKER_IMAGE
FROM ${BASE_DOCKER_IMAGE} as build

RUN mkdir -p "$GOPATH/src/github.com/erda-project/erda/"
COPY . "$GOPATH/src/github.com/erda-project/erda/"
WORKDIR "$GOPATH/src/github.com/erda-project/erda/"
RUN mkdir /tmp/dicehub-extension
RUN if [ -d "extensions" ] ; then mv extensions/* /tmp/dicehub-extension ; fi

ARG MODULE_PATH
ARG DOCKER_IMAGE
ARG MAKE_BUILD_CMD
ARG GOPROXY
RUN --mount=type=cache,target=/root/.cache/go-build\
    --mount=type=cache,target=/go/pkg/mod \
    make ${MAKE_BUILD_CMD} MODULE_PATH=${MODULE_PATH} DOCKER_IMAGE=${DOCKER_IMAGE} GOPROXY=${GOPROXY}



FROM ${BASE_DOCKER_IMAGE} as app-handler
ARG MODULE_PATH
ENV PROJ_ROOT="/go/src/github.com/erda-project/erda"
COPY --from=build "${PROJ_ROOT}" /erda
COPY --from=build /tmp/dicehub-extension /erda/conf/extensions-init
WORKDIR /erda
# handle app
RUN ./build/scripts/build_all/app_handler.sh /erda "${MODULE_PATH}" /erda-handled



FROM ${BASE_DOCKER_IMAGE}
ARG MODULE_PATH
ENV MODULE_PATH=${MODULE_PATH}

# use for ops
RUN curl -o /usr/bin/orgalorg https://terminus-dice.oss.aliyuncs.com/installer/orgalorg && \
    chmod 755 /usr/bin/orgalorg

RUN \
    npm i -g jackson-converter@1.0.10 && \
    pip3 install dicttoxml xmindparser

COPY --from=app-handler /erda-handled /erda

# use for gittar
COPY --from=build "/go/src/github.com/erda-project/erda/build/dockerfiles/gittar-resource/.gitconfig" "/root/.gitconfig"

WORKDIR /erda
CMD ["bash", "-c", "/erda/cmd/${MODULE_PATH}/bin"]
