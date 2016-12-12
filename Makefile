all: BTClient.go
	GOOS=windows GOARCH=386 go build -o build/win/BTClient.exe
	GOOS=windows GOARCH=amd64 go build -o build/win/BTClient-x64.exe
	
	GOOS=darwin GOARCH=386 CGO_ENABLED=1 go build -o build/mac/BTClient
	GOOS=darwin GOARCH=amd64 go build -o build/mac/BTClient-x64

	GOOS=linux GOARCH=386 go build -o build/linux/BTClient
	GOOS=linux GOARCH=amd64 go build -o build/linux/BTClient-x64
clean:
	rm -rf build
