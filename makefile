test:
	go test -count=1 ./...

testv:
	go test -race -v -count=1 ./...
