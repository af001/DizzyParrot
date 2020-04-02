# DizzyParrot
A simple beaconing implant written in Go. DizzyParrot consist of an agent, listening post, and interactive shell. The agent communicates to the LP over HTTP/2 at a specified interval. An agent can be tasked using the interactive shell to pull and push files, and execute commands on the target host. 

## Getting Started

#### Install dependencies (Debian and Ubuntu)
```
apt update && apt -y upgrade && apt -y install build-essential postgresql screen vim
```

#### Install go
```
wget https://dl.google.com/go/go1.14.1.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.14.1.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.profile
source ~/.profile
```

#### Clone the repo using git
```
git clone https://github.com/af001/DizzyParrot.git
```

#### Build LP and shell
```
cd server; go build server.go
cd shell; go build shell.go
```

#### Build agent 
```
# Build for host Linux
go build -ldflags "-X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443//portal/details"

# Cross-compile mips
GOOS=linux GOARCH=mips go build -ldflags "-X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443//portal/details"
```


