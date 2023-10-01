FROM golang AS builder

WORKDIR /src
COPY . .
ENV CGO_ENABLED=0
RUN go build -v

FROM alpine

COPY --from=builder /src/tibberevmqtt /bin/tibberevmqtt

ENTRYPOINT ["/bin/tibberevmqtt"]
