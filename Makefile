
all: server worker

server:
	go build -o build/server.exe ./cmd/server

worker:
	go build -o build/worker.exe ./cmd/worker