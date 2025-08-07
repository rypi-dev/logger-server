# Dockerfile
FROM golang:1.22-alpine

# Installer sqlite et ca-certificates
RUN apk add --no-cache sqlite ca-certificates

WORKDIR /app

# Copier les fichiers nécessaires
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

# Compiler l'application
RUN go build -o logger-server ./cmd

# Port exposé
EXPOSE 8080

# Dossier pour stocker la base de données
VOLUME ["/app/data"]

# Lancer le serveur
CMD ["./logger-server"]
