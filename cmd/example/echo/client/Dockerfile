ARG PROJECT_NAME=yapr_echo_client

# build stage
FROM golang:1.22 as builder
ARG PROJECT_NAME

WORKDIR /app

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o ${PROJECT_NAME} cmd/example/echo/client/main.go

# target stage
FROM alpine:latest
ARG PROJECT_NAME
RUN apk --no-cache add ca-certificates

WORKDIR /app

ENV TZ=Asia/Shanghai
RUN apk add --no-cache tzdata && ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY --from=builder /app/${PROJECT_NAME} ${PROJECT_NAME}

CMD ["./${PROJECT_NAME}"]