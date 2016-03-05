package main

import (
	"encoding/hex"
	"log"
	"os"
	"strings"

	"github.com/StreamBoat/kodi_jsonrpc"
	"github.com/tarm/serial"
)

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

func main() {
	kodi_jsonrpc.SetLogLevel(kodi_jsonrpc.LogInfoLevel)

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

	for code := range codes {
		log.Printf("got code: %s", code)
	}
}
