FROM golang:1.26 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/mock-backend ./cmd/mock-backend

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=build /out/mock-backend /app/mock-backend
COPY rmqiuwpath /app/rmqiuwpath

ENV RMQIW_PATH=/app/rmqiuwpath

ENTRYPOINT ["/app/mock-backend"]
