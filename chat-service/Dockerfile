ARG GO_IMAGE=m.daocloud.io/docker.io/library/golang:1.25.2-alpine
ARG RUNTIME_IMAGE=m.daocloud.io/docker.io/library/alpine:3.20

FROM ${GO_IMAGE} AS builder

WORKDIR /src

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off

COPY chat-service/ ./chat-service/
COPY shared/ ./shared/
RUN printf "go 1.25.2\n\nuse (\n\t./chat-service\n\t./shared\n)\n" > ./go.work
RUN mkdir -p /out
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./chat-service/cmd/server

FROM ${RUNTIME_IMAGE}

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app

COPY --from=builder /out/server /app/server

EXPOSE 9003

USER app

ENTRYPOINT ["/app/server"]
