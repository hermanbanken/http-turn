build: http-turn

http-turn: http-turn-linux http-turn-darwin

http-turn-linux:
	env GOOS=linux GOARCH=amd64 go build -o http-turn-linux ./

http-turn-darwin:
	env GOOS=darwin GOARCH=amd64 go build -o http-turn-darwin ./
