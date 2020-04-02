# Go Parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_SERVER=server
BINARY_CLIENT=client
BINARY_SHELL=shell
FLAGS="-s -w"

# Client Settings
URL=https://localhost:8443/portal/status
SECRET=dizzyparrot
CALLBACK=300
JITTER=60
NAME=A10000

LDFLAGS="-s -w -X main.name=$(NAME) -X main.url=$(URL) -X main.callback=$(CALLBACK) -X main.jitter=$(JITTER) -X main.secret=$(SECRET)"

all: test build
build:
	cd server; $(GOBUILD) -o $(BINARY_SERVER) -v -ldflags $(FLAGS); cd ../
	cd client; $(GOBUILD) -o $(BINARY_CLIENT) -v -ldflags $(LDFLAGS); cd ../
	cd shell; $(GOBUILD) -o $(BINARY_SHELL) -v -ldflags $(FLAGS); cd ../
test:
	cd server; $(GOTEST) -v ./... ; cd ../
	cd client; $(GOTEST) -v ./... ; cd ../
	cd shell; $(GOTEST) -v ./... ; cd ../
clean:
	$(GOCLEAN)
	rm -f server/$(BINARY_SERVER)
	rm -f client/$(BINARY_CLIENT)
	rm -f shell/$(BINARY_SHELL)
	rm -f client/$(BINARY_CLIENT)_arm
	rm -f client/$(BINARY_CLIENT)_ppc
	rm -f client/$(BINARY_CLIENT)_arm
	rm -f client/$(BINARY_CLIENT)_linux64
build_server:
	cd server; $(GOBUILD) -o $(BINARY_SERVER) -v -ldflags $(FLAGS); cd ../
build_shell:
	cd shell; $(GOBUILD) -o $(BINARY_SHELL) -v -ldflags $(FLAGS); cd ../
build_linux64:
	cd client; GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_CLIENT)_linux64 -v -ldflags $(LDFLAGS); cd ../
build_mips:
	cd client; GOOS=linux GOARCH=mips $(GOBUILD) -o $(BINARY_CLIENT)_mips -v -ldflags $(LDFLAGS); cd ../
build_ppc:
	cd client; GOOS=linux GOARCH=ppc64 $(GOBUILD) -o $(BINARY_CLIENT)_ppc -v -ldflags $(LDFLAGS); cd ../
build_arm:
	cd client; GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) -o $(BINARY_CLIENT)_arm -v -ldflags $(LDFLAGS); cd ../
deps:
	$(GOGET) gopkg.in/yaml.v2
	$(GOGET) golang.org/x/net/http2
	$(GOGET) github.com/lib/pq
	$(GOGET) github.com/c-bata/go-prompt
	$(GOGET) github.com/common-nighthawk/go-figure
	$(GOGET) github.com/jedib0t/go-pretty/table
