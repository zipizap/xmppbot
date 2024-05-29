package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mattn/go-xmpp"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Server   string   `yaml:"server"`
	User     string   `yaml:"user"`
	Password string   `yaml:"password"`
	Contacts []string `yaml:"contacts"`
	Rules    []Rule   `yaml:"rules"`
}

type Rule struct {
	Regexp         string `yaml:"regexp"`
	BinaryFilepath string `yaml:"binaryFilepath"`
}

func main() {
	config := getConfig()

	options := xmpp.Options{
		Host:     config.Server,
		User:     config.User,
		Password: config.Password,
		// NOTLS true allows StartTls to happen and upgrade the connection from unencrypted to TLS-encrypted
		NoTLS: true,
	}

	client, err := options.NewClient()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Listening for messages")
	handlerCounter := 1
	for {
		message, err := client.Recv()
		if err != nil {
			logrus.Error(err)
			continue
		}

		switch v := message.(type) {
		case xmpp.Chat:
			if v.Text == "" {
				continue
			}
			if !contains(config.Contacts, v.Remote) {
				continue
			}
			go handleMessage(v.Text, v.Remote, config.Rules, client, handlerCounter)
			handlerCounter += 1
		case xmpp.Presence:
			continue
		default:
			//logrus.Warnf("Unexpected message type: %T", v)
			continue
		}
	}
}

func handleMessage(message string, fromContact string, rules []Rule, client *xmpp.Client, handlerCounter int) {
	logrus.Infof("[%d] Received message from '%s':\n%s\n", handlerCounter, fromContact, ident(message, "<--- "))
	for _, rule := range rules {
		matched, err := regexp.MatchString(rule.Regexp, message)
		if err != nil {
			logrus.Error(err)
			continue
		}

		if matched {
			reply := fmt.Sprintf("[%d] Processing command: '%s'\n", handlerCounter, message)
			client.Send(xmpp.Chat{Type: "chat", Text: reply, Remote: fromContact})
			logrus.Infof("[%d] Sent reply:\n%s\n", handlerCounter, ident(reply, "---> "))

			cmd := exec.Command(rule.BinaryFilepath, message)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logrus.Error(err)
				break
			}

			exitCode := cmd.ProcessState.ExitCode()
			reply = fmt.Sprintf("%s\n[%d]---[Exit-code: %d]-------------\n", output, handlerCounter, exitCode)
			//client.Send(xmpp.Chat{Type: "chat", Text: reply})
			client.Send(xmpp.Chat{Type: "chat", Text: reply, Remote: fromContact})
			logrus.Infof("[%d] Sent reply:\n%s\n", handlerCounter, ident(reply, "---> "))
			break
		}
	}
}

func contains(slice []string, item string) bool {
	for _, element := range slice {
		matched, err := regexp.MatchString(element, item)
		if err != nil {
			logrus.Fatal(err)
			return false
		}
		if matched {
			return true
		}
	}
	return false
}

func getConfig() Config {
	data, err := ioutil.ReadFile("./xmppbot.config.yaml")
	if err != nil {
		logrus.Fatal(err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		logrus.Fatal(err)
	}

	return config
}

func ident(input string, identChars string) string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = identChars + line
	}
	return strings.Join(lines, "\n")
}
