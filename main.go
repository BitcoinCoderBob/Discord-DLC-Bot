// The MIT License (MIT)
// Copyright © 2021 Bitcoincoderbob

// Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	Token string
)

func init() {

	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

type DLC struct {
	name    string
	date    string
	options []string
}

var suggestedDLCs []DLC

func main() {
	// https://bitcoin-s.org/docs/next/oracle/oracle-election-example
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	dg.AddHandler(messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	fmt.Printf("message: %v\n", m.Content)

	if strings.Contains(m.Content, "accept ") {
		name := strings.Fields(m.Content)[1]
		for _, dlc := range suggestedDLCs {
			if dlc.name == name {
				fmt.Println("that is true")
				s.ChannelMessageSend("905993492835237912", "Executing DLC...")
				// execute dlc
				dlcAnnouncement, err := announceDLC(dlc)
				if err != nil {
					s.ChannelMessageSend("905993492835237912", fmt.Sprintf("Error announcing DLC: %s", err.Error()))
					return
				}
				s.ChannelMessageSend("905993492835237912", fmt.Sprintf("createenumannouncement: %s", dlcAnnouncement))
				return
			}
		}
		retMsg := fmt.Sprintf("DLC with name %s not found", name)
		s.ChannelMessageSend("905993492835237912", retMsg)
	} else if strings.Contains(m.Content, "check ") {
		dlcName := strings.Fields(m.Content)[1]
		for _, dlc := range suggestedDLCs {
			if dlc.name == dlcName {
				winner, err := checkDLC()
				if err != nil {
					s.ChannelMessageSend("905993492835237912", fmt.Sprintf("Error getting event data: %s", err.Error()))
					return
				}
				sig, err := signWinner(dlcName, winner)
				if err != nil {
					s.ChannelMessageSend("905993492835237912", fmt.Sprintf("Error signing winner: %s", err.Error()))
					return
				}
				winningMessage := fmt.Sprintf("WINNING EVENT: %s\nsignenum: %s", winner, sig)
				s.ChannelMessageSend("905993492835237912", winningMessage)
				announcement, err := getannouncement(dlcName)
				if err != nil {
					s.ChannelMessageSend("905993492835237912", fmt.Sprintf("Error with getannouncement with dlc %s: %s", dlcName, err.Error()))
					return
				}
				s.ChannelMessageSend("905993492835237912", fmt.Sprintf("Announcement: %s", announcement))
				return
			}
		}
		retMsg := fmt.Sprintf("DLC with name %s not found", dlcName)
		s.ChannelMessageSend("905993492835237912", retMsg)
	}
	//dlc eventName eventTime [option1,option2,option3...]
	name, date, options, err := parsedlc(m.Content)
	if err != nil {
		fmt.Printf("Error with dlc: %s\n", err.Error())
		return
	}
	suggestedDLCs = append(suggestedDLCs, setDlc(name, date, options))
}

func announceDLC(dlc DLC) (announcement string, err error) {
	// /home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux createenumannouncement 2020-us-election1 "2022-01-20T00:00:00Z" "Republican_win,Democrat_win,other" --network testnet3 --rpcport 9998
	fmt.Println("EXECUTING DLC")
	fmtOptions := ""
	for index, o := range dlc.options {
		if index == 0 {
			fmtOptions += "\""
		}
		if index == len(dlc.options)-1 {
			fmtOptions += o + "\""
			break
		}
		fmtOptions += o + ","
	}
	fmt.Println(fmtOptions)
	cmd := exec.Command("/home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux", "createenumannouncement", dlc.name, dlc.date, fmtOptions, "--network", "testnet3", "--rpcport", "9998")
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("err w combinedoutput: %s\n", err.Error())
	}
	fmt.Printf("output: %s\n", string(output))
	return string(output), nil
}

func signWinner(dlcName, winner string) (sig string, err error) {
	// /home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux signenum 2020-us-election1 Democrat_win --network testnet3 --rpcport 9998
	cmd := exec.Command("/home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux", "signenum", dlcName, winner, "--network", "testnet3", "--rpcport", "9998")
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("err w combinedoutput: %s\n", err.Error())
	}
	fmt.Printf("output: %s\n", string(output))
	return string(output), nil
}

func checkDLC() (name string, err error) {
	resp, err := http.Get("https://mempool.space/api/blocks/tip/hash")
	if err != nil {
		log.Fatalln(err)
	}
	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	//Convert the body to type string
	hash := string(body)
	lastDigit := fmt.Sprintf(string(hash[len(hash)-1]))
	return lastDigit, nil
}

func getannouncement(dlcName string) (ann string, err error) {
	// /home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux getannouncement 2020-us-election1 --network testnet3 --rpcport 9998
	cmd := exec.Command("/home/bob/Downloads/bitcoin-s-cli-x86_64-pc-linux", "getannouncement", dlcName, "--network", "testnet3", "--rpcport", "9998")
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("err w combinedoutput: %s\n", err.Error())
	}
	fmt.Printf("output: %s\n", string(output))
	return string(output), nil
}

func setDlc(name, date string, options []string) (dlc DLC) {
	return DLC{
		name:    name,
		date:    date,
		options: options,
	}
}

func parsedlc(content string) (name, date string, options []string, err error) {
	res := strings.Fields(content)
	name = ""
	date = ""
	options = []string{}
	for index, s := range res {
		if index == 0 && s != "dlc" {
			return "", "", nil, fmt.Errorf("not a dlc")
		}
		if index == 1 {
			name = s
		}
		if index == 2 {
			date = s
		}

		if index == 3 {
			if s[0] == 91 && s[len(s)-1] == 93 {
				optionString := s[1 : len(s)-1]
				options = strings.Split(optionString, ",")
			}
			fmt.Println("set the dlc")
			return
		}
	}
	return "", "", nil, fmt.Errorf("invalid dlc")
}
