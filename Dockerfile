FROM golang:1.13 AS builder
WORKDIR /kubernoisy
COPY . /kubernoisy
RUN go build -o kubernoisy .

FROM debian:stable-slim
WORKDIR /kubernoisy
COPY --from=builder /kubernoisy .
ENTRYPOINT ["./kubernoisy"]