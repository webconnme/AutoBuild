/**
 * The MIT License (MIT)
 *
 * Copyright (c) 2015 David You <david@webconn.me>
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
	"os"
	"log"
	"io/ioutil"
	"encoding/json"
	"encoding/base64"
	"net/smtp"
)

import (
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"

	"github.com/scorredoira/email"
	"github.com/jaytaylor/html2text"
)

type GmailManager struct {

	email  string
	ctx    context.Context

	config *oauth2.Config
	token  *oauth2.Token

	srv    *gmail.Service
	msgs   []*GmailMessage
}

func NewGmailManager(email string) GmailManager {

	gm := GmailManager{}

	gm.ctx = context.Background()
	gm.email = email
	gm.msgs = []*GmailMessage{}

	return gm
}

func (gm *GmailManager) GetConfig(client_secret_file string) *oauth2.Config {

	b, err := ioutil.ReadFile(client_secret_file)
	if err != nil {
		log.Fatalf("Unable to read %s: \n    %v", client_secret_file, err)
	}

	gm.config, err = google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		log.Fatalf("Unable to parse %s: \n    %v", client_secret_file, err)
	}

	return gm.config

}

func (gm *GmailManager) LoadToken(token_file string) *oauth2.Token {

	f, err := os.Open(token_file)
	if err != nil {
		log.Fatalf("Unable to read %s: \n    %v", token_file, err)
	}

	gm.token = &oauth2.Token{}
	err = json.NewDecoder(f).Decode(gm.token)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to parse %s: \n    %v", token_file, err)
	}

	return gm.token
}

func (gm *GmailManager) GetService() *gmail.Service {

	client := gm.config.Client(gm.ctx, gm.token)
	srv, err := gmail.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve gmail Client %v", err)
	}

	gm.srv = srv
	return gm.srv
}

func (gm *GmailManager) BuildService(client_secret_file, token_file string) {

	gm.GetConfig(client_secret_file)
	gm.LoadToken(token_file)
	gm.GetService()

}

func (gm *GmailManager) GetMailList() []*GmailMessage {

	req := gm.srv.Users.Messages.List(gm.email).LabelIds("INBOX", "UNREAD").Q("subject:[build]")
	//	req := gm.srv.Users.Messages.List(gm.email).LabelIds("INBOX")
	res, err := req.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve messages: %v", err)
	}

	gm.msgs = nil;

	for _, m := range res.Messages {
		msg := NewGmailMessage(m.Id)
		msg.gm = gm

		gm.msgs = append(gm.msgs, &msg)
	}

	return gm.msgs

}

type GmailMessage struct {
	gm      *GmailManager

	Id      string
	Subject string
	Sender  string
	Body    string
}


func NewGmailMessage(id string) GmailMessage {

	m := GmailMessage{}
	m.Id = id

	return m
}

func ( m *GmailMessage ) getHeaderValue(headers []*gmail.MessagePartHeader, name string) string {
	for _, one := range headers {
		if one.Name == name {
			return one.Value
		}
	}

	return ""
}

func ( m *GmailMessage ) getSubject(headers []*gmail.MessagePartHeader) string {
	return m.getHeaderValue(headers, "Subject")
}

func ( m *GmailMessage ) getSender(headers []*gmail.MessagePartHeader) string {
	return m.getHeaderValue(headers, "From")
}

func ( m *GmailMessage ) getBody(parts []*gmail.MessagePart) string {

	for _, part := range parts {
		if len(part.Parts) > 0 {
			return m.getBody(part.Parts)
		} else {
			if part.MimeType == "text/html" {
				return part.Body.Data
			}
		}
	}

	return ""
}

func ( m *GmailMessage) GetMail() {

	req := m.gm.srv.Users.Messages.Get(m.gm.email, m.Id)
	res, err := req.Format("full").Do()
	if err != nil {
		log.Fatalf("Unable to get messages: %v", err)
	}

	m.Subject = m.getSubject(res.Payload.Headers)
	m.Sender = m.getSender(res.Payload.Headers)
	m.Body = m.getBody(res.Payload.Parts)
}

func ( m *GmailMessage) RemoveLabel(label ...string) {

	_, err := m.gm.srv.Users.Messages.Modify(
		m.gm.email,
		m.Id,
		&gmail.ModifyMessageRequest{
			RemoveLabelIds: []string{"UNREAD"}}).Do()

	if err != nil {
		log.Println(m.Id, ">>", "RemoveLabel Error", err)
	} else {
//		log.Println(m.Id, ">>", "RemoveLabel", msg)
	}
}

func ( m *GmailMessage) GetBodyHTML() string {

	html, _ := base64.URLEncoding.DecodeString(m.Body)
	//	html   := base64.StdEncoding.EncodeToString(data)

	return string(html)
}

func ( m *GmailMessage) GetBodyTEXT() string {

	text, _ := html2text.FromString(m.GetBodyHTML())

	return string(text)

}

func ( m *GmailMessage) Reply(to, subject, body string) {
	em := email.NewMessage(subject, body)
	em.From = cfg.Id
	em.To = []string{ to }

	err := em.Attach(m.Id + "/autobuild.log")
	if err != nil {
		log.Println(err)
	}

	err = email.Send(cfg.SmtpFull, smtp.PlainAuth("", cfg.Id, cfg.Password, cfg.Smtp), em)
	if err != nil {
		log.Println(err)
	}
}