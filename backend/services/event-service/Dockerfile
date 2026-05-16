FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /out/event-api ./cmd/api
RUN go build -o /out/booking-worker ./cmd/worker

FROM alpine:3.21

WORKDIR /app
COPY --from=build /out/event-api /usr/local/bin/event-api
COPY --from=build /out/booking-worker /usr/local/bin/booking-worker

CMD ["event-api"]
