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
	"time"
	"github.com/tbruyelle/hipchat-go/hipchat"
	"os"
	"regexp"
	"os/exec"
)

var config struct {
	HipchatAuthToken string `json:"hipchat_auth_token"`
	Password string `json:"password"`
	RoomNames []string `json:"room_names"`
	Botname string `json:"botname"`
	UnknownCommand string `json:"unknown_command"`
	NoAuthMessage string `json:"no_auth_message"`
	MessageCheckFrequency time.Duration `json:"message_check_frequency_seconds"`
	Scripts []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		PermittedUsers []string `json:"permitted_users"`
		HelpText string `json:"help_text"`
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
	last_message_recieved := time.Now().Format("2006-01-02T15:04:05Z07:00")

	fmt.Println("Gobot running ctrl+c exits")

	for {

		for _, room := range config.RoomNames {

			//fmt.Println("LAST MESSAGE RECIEVED")
			//fmt.Println(last_message_recieved)

			//2015-03-13T10:05:59.075615+00:00
			//2015-03-13T10:05:59.075615+00:00

			//add 1 second to last date retrived from room then write to an array on second loop through use that date
			//when initiall firing up the script start date is beginning of the day

			history, response, error :=
				client.Room.History(room, &hipchat.HistoryRequest{Date: last_message_recieved,
					                                              Timezone: "UTC"})

			if error != nil {
				fmt.Printf("Error during room history req %q\n", error)
				fmt.Printf("Server returns %+v\n", response)

				return
			}

			for _, m := range history.Items {
				from := ""
				message_directed_at_bot := false

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

				if message_directed_at_bot {
					command := re.ReplaceAllString(m.Message, "")
					script_to_execute := ""
					user_has_permissions := false

					//command needs to be split into command and variable number of arguments

					if command == "help" {
						fmt.Println("Help message")
					} else {
						for _, script := range config.Scripts {
							if command == script.Name {
								script_to_execute = script.Path

								for _, user := range script.PermittedUsers {
									if user == "all" || user == from {
										user_has_permissions = true
									}
								}

							}
						}

						if len(script_to_execute) > 0 && user_has_permissions {
							cmd := exec.Command(script_to_execute + " &")
							error := cmd.Start()

							//output script error into hipchat

							if error != nil {
								client.Room.Notification(room,
									&hipchat.NotificationRequest{Color: "red",
									                             Message: "@" + from + " " + command + " failed to execute.",
								                                 Notify: true, MessageFormat: "text"})
								message = "@" + from + " " + command + " failed to execute."
							}

						} else {
							message := ""

							if len(script_to_execute) > 0 {
								message = re_name.ReplaceAllString(config.NoAuthMessage, "@" + from)
							} else {
								message = re_name.ReplaceAllString(config.UnknownCommand, "@" + from)
							}

							client.Room.Notification(room,
								&hipchat.NotificationRequest{Color: "red", Message: message,
								                             Notify: true, MessageFormat: "text"})
						}
					}
				}

				last_message_recieved = m.Date

			}

		}

		time.Sleep(config.MessageCheckFrequency)

	}

}