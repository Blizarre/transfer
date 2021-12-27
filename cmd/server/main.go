package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"time"
)

func generateID() string {
	return fmt.Sprintf("%v", rand.Uint32()%100000)
}

func getURL(listen_http_str string, id string) string {
	return fmt.Sprintf("http://%v/%v/file.bin", listen_http_str, id)
}

func transferServer(c net.Conn, listener chan io.Writer, id string, listen_http_str string) {
	messageToSender := func(message string) error {
		log.Println(id, "to Sender:", message)
		_, err := c.Write([]byte(message + "\n"))
		if err != nil {
			log.Println("Write error to sender", err.Error())
		}
		return err
	}

	defer func() {
		// Errors will be logged. There is nothing else to do
		messageToSender("Closing")

		err := c.Close()
		if err != nil {
			log.Println(id, "error closing", err)
		}
		listener <- nil
	}()

	url := getURL(listen_http_str, id)
	if messageToSender(url) != nil {
		return
	}

	log.Println(id, "Serving", url, "- waiting for a connection")

	ticker_30s := time.NewTicker(30 * time.Second)
	ticker_90m := time.NewTicker(90 * time.Minute)

	var receiver io.Writer = nil

	for receiver == nil {
		select {
		case receiver = <-listener:
			break
		case <-ticker_30s.C:
			if messageToSender("Waiting") != nil {
				return
			}
		case <-ticker_90m.C:
			messageToSender("No receiver found, Stopping")
			return
		}
	}
	if receiver == nil {
		messageToSender("Error on the receiver side")
		return
	}

	ticker_30s.Stop()
	ticker_90m.Stop()

	if messageToSender("Starting transfer") != nil {
		return
	}

	buffer := make([]byte, 4096)

	for {
		nr, errRead := c.Read(buffer)
		if errRead != nil && errRead != io.EOF {
			// Errors will be logged. There is nothing else to do
			messageToSender("Error reading data from the sender")
			log.Println(id, "Error while reading data", errRead.Error())
			return
		}
		log.Println(id, "Read", nr)

		_, err := receiver.Write(buffer[:nr])
		if err != nil {
			// Errors will be logged. There is nothing else to do
			messageToSender("Error writing data to the receiver")
			log.Println(id, "Error while Writing data", err.Error())
			return
		}

		if errRead == io.EOF {
			log.Println(id, "Transfer Successful")
			return
		}
	}

}

func sendData(mapping *SafeMap, w FlushWriter, path string) {
	log.Println("Serving", path)
	splits := strings.Split(path, "/")
	if len(splits) != 3 {
		log.Println("Error: Invalid path", path)
		w.w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("File not found"))
		return
	}
	id := splits[1]

	conn, ok := mapping.Pop(id)
	if !ok {
		w.w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("File not found"))
	} else {
		w.w.WriteHeader(http.StatusAccepted)
		conn <- w
		<-conn
	}
}

func main() {
	listen_str := flag.String("host-receiver", "localhost:1234", "Definition for the receiver")
	listen_http_str := flag.String("host-sender", "localhost:8080", "Definition for the host")
	flag.Parse()

	mapping := NewSafeMap()

	http.HandleFunc("/", func(writer http.ResponseWriter, r *http.Request) {
		flusher, ok := writer.(http.Flusher)
		if !ok {
			log.Panicln("Error, un-chunkable")
			return
		}

		w := FlushWriter{
			f: flusher,
			w: writer,
		}

		sendData(&mapping, w, r.URL.Path)
	})

	l, err := net.Listen("tcp4", *listen_str)
	if err != nil {
		log.Println("listen error", err.Error())
		return
	}
	log.Println("Receiver server started, listening to", *listen_str)

	go func() {
		log.Println("Sender server started, listening to", *listen_http_str)
		err := http.ListenAndServe(*listen_http_str, nil)
		log.Fatal("Error during http listening", err)
	}()

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Println("accept error", err.Error())
			return
		}
		channel := make(chan io.Writer)
		id := generateID()
		mapping.Add(id, channel)
		go transferServer(fd, channel, id, *listen_http_str)
	}
}
