# Stage 1: Build Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /skynetd ./cmd/skynetd

# Stage 2: Build Dashboard
FROM node:22-alpine AS frontend

WORKDIR /app
COPY web/dashboard/package.json web/dashboard/package-lock.json ./
RUN npm ci
COPY web/dashboard/ ./
RUN npm run build

# Stage 3: Final image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /skynetd .
COPY --from=frontend /app/dist ./dashboard
COPY config/ ./config/
COPY migrations/ ./migrations/

EXPOSE 9090

CMD ["./skynetd"]
