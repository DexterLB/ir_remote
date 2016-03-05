package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/tarm/serial"
)

type Mpv struct {
	filename string
	client   net.Conn
}

func NewMpv(filename string) *Mpv {
	m := &Mpv{
		filename: filename,
	}
	return m
}

func (m *Mpv) Connect() error {
	var err error

	m.client, err = net.Dial("unix", m.filename)
	if err != nil {
		return err
	}
	return nil
}

func (m *Mpv) TrySend(data string) bool {
	var err error
	if m.client == nil {
		err = m.Connect()
		if err != nil {
			log.Printf("can't connect to mpv: %s", err)
			return false
		}
	}
	err = m.Send(data)
	if err != nil {
		m.client.Close()
		err = m.Connect()
		if err != nil {
			log.Printf("mpv connection dropped: %s", err)
			m.client = nil
			return false
		}
		err = m.Send(data)
		if err != nil {
			log.Printf("can't send data to mpv: %s", err)
			return false
		}
	}
	return true
}

func (m *Mpv) Send(data string) error {
	_, err := m.client.Write([]byte(data))
	return err
}

func readCodesMega8(filename string, codes chan<- string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	data := make([]byte, 7)

	for {
		_, err = f.Read(data)
		if err != nil {
			return err
		}
		codes <- hex.EncodeToString(data)
	}
}

func readCodesAt90(filename string, codes chan<- string) error {
	f, err := serial.OpenPort(&serial.Config{
		Name: filename,
		Baud: 9600,
	})
	if err != nil {
		return err
	}

	buf := make([]byte, 128)

	var (
		n           int
		data        string
		currentCode string
	)

	crReplacer := strings.NewReplacer("\r", "\n")

	for {
		n, err = f.Read(buf)
		if err != nil {
			return err
		}
		data += crReplacer.Replace(string(buf[:n]))

		portions := strings.Split(data, "\n")
		for _, portion := range portions[:len(portions)-1] {
			message := strings.Trim(portion, " \n")
			if len(message) > 0 {
				if message == "OK" {
					codes <- currentCode
				} else {
					currentCode = message
				}
			}
		}
		data = portions[len(portions)-1]
	}
}

func readRemotes() chan string {
	codes := make(chan string)
	go func() {
		err := readCodesAt90("/dev/at90_ir_reader", codes)
		if err != nil {
			log.Printf("End of stream: %s", err)
		}
	}()

	go func() {
		err := readCodesMega8("/dev/mega8_vusb_ir_reader", codes)
		if err != nil {
			log.Printf("End of stream: %s", err)
		}
	}()

	return codes
}

func getCommandMap(filename string) (map[string]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	commands := make(map[string]string)

	err = decoder.Decode(&commands)
	if err != nil {
		return nil, err
	}

	return commands, nil
}

func main() {
	commands, err := getCommandMap("commands.json")
	if err != nil {
		log.Fatalf("can't read command map: %s", err)
	}

	mpv := NewMpv("/tmp/mpv_rpc")
	codes := readRemotes()

	for code := range codes {
		command, ok := commands[code]
		if ok {
			log.Printf("got code %s: command %s", code, command)
			mpv.TrySend(fmt.Sprintf("%s\n", command))
		} else {
			log.Printf("got unknown code %s", code)
		}
	}
}
