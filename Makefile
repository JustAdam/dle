GITTAG = `git describe --tags --abbrev=0 | sed 's/^v//' | sed 's/\+.*$$//'`

.PHONEY: clean

test:
	go vet ./...
	gb test

clean:
	go clean ./...

build: clean
	gb build -ldflags "-s"

docker: build
	mv bin/dle Dockerfiles
	cd Dockerfiles
	docker build -t justadam/dle:$(GITTAG) .
