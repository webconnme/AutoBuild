/**
 * The MIT License (MIT)
 *
 * Copyright (c) 2015 Victor Kim <victor@webconn.me>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"log"
	"encoding/json"
	"os"
	"os/exec"
	"fmt"
	"time"
	"io/ioutil"
	"github.com/kennygrant/sanitize"
)

const VERSION = "0.9.0"

var cfg struct {
	Id string `json:"id"`
	Password string `json:"password"`
	SmtpFull string `json:"smtpFull"`
	Smtp string `json:"smtp"`
}

func main() {
	log.Println("VERSION", VERSION)

	email := "me"
	gm := NewGmailManager(email)
	gm.BuildService("client_secret.json", "token.json")

	config("config.json")
	
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

func config(filename string) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
	}
	
	err = json.Unmarshal(bytes, &cfg)
	if err != nil {
		log.Fatalln(err)
	}
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
//	defer os.RemoveAll(id)

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
