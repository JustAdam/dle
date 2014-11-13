TARGET = Dockerfiles/dle
GITTAG = `git describe --tags --abbrev=0 | sed 's/^v//' | sed 's/\+.*$$//'`

.PHONEY: getdeps clean install

getdeps: 
	source gvp
	gmp install

test:
	go vet
	go test -covermode=count ./...

clean:
	rm -f $(TARGET)
	go clean

build: clean
	go build -a -ldflags "-s" -o $(TARGET) dle.go tls.go

docker: build
	chmod +x $(TARGET)
	cd Dockerfiles
	docker build -t justadam/dle:$(GITTAG) .