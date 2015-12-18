package main

import (
	"log"
	"encoding/json"
	"os"
	"os/exec"
	"fmt"
	"github.com/kennygrant/sanitize"
	"time"
)

func main() {
	email := "me"
	gm := NewGmailManager(email)
	gm.BuildService("client_secret.json", "token.json")


	log.Println("Start")
	for {
		list := gm.GetMailList()
		log.Println("List", len(list))
		for _, m := range list {
			go process(m)
		}

		time.Sleep(1 * time.Minute)
	}
	log.Println("End")
}

type Task struct {
	From   string `json:"from"`
	To     string `json:"to"`
}

func process(m *GmailMessage) {
	id := m.Id

	defer log.Println(id, ">>", "Done")

	log.Println(id, ">>", "Get")
	m.GetMail()

	log.Println(id, ">>", "Unlabel")
	m.RemoveLabel("UNREAD")

	log.Println(id, ">>", "Parse")
	body := sanitize.HTML(m.GetBodyHTML())

	log.Println(id, ">>", "Unmarshal")
	var task Task
	err := json.Unmarshal([]byte(body), &task)
	if err != nil {
		log.Println(id, ">>", "Error", err)
		m.Reply(task.To, "[build/result] FAIL",
			fmt.Sprintf("Requested Build from %v is failed\n\n%v\n\n%v\n\n", task.From, err, body))
		return
	}

	log.Println(id, ">>", "Mkdir")
	err = os.Mkdir(id, os.ModePerm)
	if err != nil {
		log.Println(id, ">>", "Error", err)
		m.Reply(task.To, "[build/result] FAIL",
			fmt.Sprintf("Requested Build from %v is failed\n\n%v\n\n%v\n\n", task.From, err, body))
		return
	}
	defer os.RemoveAll(id)

	log.Println(id, ">>", "Run", body)
	cmd := exec.Command("../autobuild", body)
	cmd.Dir = id

	bytes, err := cmd.CombinedOutput()

	str := string(bytes)
	log.Println(id, ">>", "Log", str)

	if err != nil {
		log.Println(id, ">>", "Error", err)
		m.Reply(task.To, "[build/result] FAIL",
			fmt.Sprintf("Requested Build from %v is failed\n\n%v\n\n%v\n\n", task.From, err, body))
		return
	}

	m.Reply(task.To, "[build/result] SUCCESS",
		fmt.Sprintf("Requested Build from %v is done successfully\n\n", task.From))
}