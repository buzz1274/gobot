/*
Copyright (c) 2015 David Exelby
Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
associated documentation files (the "Software"), to deal in the Software without restriction,
including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense,
and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:
The above copyright notice and this permission notice shall be included in all copies or substantial
portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT
LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/tbruyelle/hipchat-go/hipchat"
)

var config struct {
	HipchatAuthToken      string        `json:"hipchat_auth_token"`
	Password              string        `json:"password"`
	RoomNames             []string      `json:"room_names"`
	Botname               string        `json:"botname"`
	UnknownCommand        string        `json:"unknown_command"`
	NoAuthMessage         string        `json:"no_auth_message"`
	StartMessage          string        `json:"start_message"`
	MessageCheckFrequency time.Duration `json:"message_check_frequency_seconds"`
	Scripts               []struct {
		Name           string   `json:"name"`
		Path           string   `json:"path"`
		PermittedUsers []string `json:"permitted_users"`
		HelpText       string   `json:"help_text"`
	} `json:"scripts"`
}

func main() {

	configFile, err := os.Open("config.json")

	if err != nil {
		fmt.Printf("opening config file %q\n", err.Error())

		return
	}

	jsonParser := json.NewDecoder(configFile)

	if err = jsonParser.Decode(&config); err != nil {
		fmt.Printf("parsing config file %q\n", err.Error())

		return
	}

	re := regexp.MustCompile("(@.*?)\\s")
	re_name := regexp.MustCompile("\\*name\\*")
	client := hipchat.NewClient(config.HipchatAuthToken)
	last_message_recieved := time.Now()

	client.Room.Notification(config.RoomNames[0],
		&hipchat.NotificationRequest{Message: config.StartMessage,
		                             Notify: false, MessageFormat: "text"})

	for {

		for _, room := range config.RoomNames {

			history, response, error :=
				client.Room.History(room, &hipchat.HistoryRequest{Date: "recent"})

			if error != nil {
				fmt.Printf("Error during room history req %q\n", error)
				fmt.Printf("Server returns %+v\n", response)

				return
			}

			for _, m := range history.Items {
				from := ""
				message_directed_at_bot := false
				message_time_parsed, _ := time.Parse("2006-01-02T15:04:05Z07:00", m.Date)

				if message_time_parsed.After(last_message_recieved) {

					switch m.From.(type) {
					case string:
						from = m.From.(string)
					case map[string]interface{}:
						f := m.From.(map[string]interface{})
						from = f["mention_name"].(string)
					}

					if m.Mentions != nil {
						for _, mention := range m.Mentions {
							if mention.MentionName == config.Botname {
								message_directed_at_bot = true
							}
						}
					}

					fmt.Println(from)

					if message_directed_at_bot && config.Botname != strings.Replace(from, " ", "", -1) {
						command := strings.Fields(re.ReplaceAllString(m.Message, ""))
						script_to_execute := ""
						user_has_permissions := false

						if command[0] == "help" {
							fmt.Println("Help message")
						} else if command[0] == "ping" {
							client.Room.Notification(room,
								&hipchat.NotificationRequest{Message: "@" + from + " pong",
									MessageFormat: "text"})
						} else {
							for _, script := range config.Scripts {
								if command[0] == script.Name {
									script_to_execute = script.Path

									for _, user := range script.PermittedUsers {
										if user == "all" || user == from {
											user_has_permissions = true
										}
									}
								}
							}

							message := ""

							if len(script_to_execute) > 0 && user_has_permissions {
								command = append(command[1:len(command)], "&")
								cmd := exec.Command(script_to_execute, command...)
								error := cmd.Start()

								//output script error into hipchat

								if error != nil {
									client.Room.Notification(room,
										&hipchat.NotificationRequest{Color: "red",
											Message: "@" + from + " " + command[0] + " failed to execute.",
											Notify:  true, MessageFormat: "text"})
								}

							} else {
								if len(script_to_execute) > 0 {
									message = re_name.ReplaceAllString(config.NoAuthMessage, "@"+from)
								} else {
									message = re_name.ReplaceAllString(config.UnknownCommand, "@"+from)
								}

								client.Room.Notification(room,
									&hipchat.NotificationRequest{Color: "red", Message: message,
										Notify: true, MessageFormat: "text"})
							}
						}
					}

					last_message_recieved = message_time_parsed

				}
			}

		}

		time.Sleep(config.MessageCheckFrequency)

	}

}
