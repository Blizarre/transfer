package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"
)

func generateID() string {
	return fmt.Sprintf("%v", rand.Uint32()%100000)
}

func getURL(listen_http_str string, id string) string {
	return fmt.Sprintf("http://%v/%v/file", listen_http_str, id)
}

func transferServer(sender net.Conn, listener chan io.Writer, id string, listen_http_str string) {
	messageToSender := func(message string) error {
		log.Println(id, "to Sender:", message)
		_, err := sender.Write([]byte(message + "\n"))
		if err != nil {
			log.Println("Write error to sender", err.Error())
		}
		return err
	}

	status_message := "[status] Waiting"

	// Closing this channel means the end of the transfer
	quit := make(chan struct{})
	go func() {
		ticker_30s := time.NewTicker(30 * time.Second)
		defer ticker_30s.Stop()
		for {
			select {
			case <-ticker_30s.C:
				// To prevent a timeout, we send some data every 30 seconds. TCP keepalive might be enough by itself
				// for short waits, but it might not be enough to last 90 minutes. (and it is nice to get a message
				// to confirm that everything is fine).
				if messageToSender(status_message) != nil {
					close(quit)
					return
				}
			case <-quit:
				return
			}
		}
	}()

	defer func() {
		// Errors will be logged. There is nothing else to do
		messageToSender("Closing")

		err := sender.Close()
		if err != nil {
			log.Println(id, "error when closing:", err)
		}
		close(listener)
		close(quit)
	}()

	url := getURL(listen_http_str, id)
	if messageToSender(url) != nil {
		return
	}

	log.Println(id, "Serving", url, "- waiting for a connection")

	var receiver io.Writer = nil

	// Waiting up to 90 minutes for a message on the listener channel. The message will
	// contain the io.Writer used to send the HTTP response
	ticker_90m := time.NewTicker(90 * time.Minute)
	for receiver == nil {
		select {
		case receiver = <-listener:
			break
		case <-ticker_90m.C:
			messageToSender("No receiver found, Stopping")
			return
		}
	}
	ticker_90m.Stop()

	if receiver == nil {
		messageToSender("Error on the receiver side")
		return
	}

	if messageToSender("Starting transfer") != nil {
		return
	}

	status_message = "[status] Transferring data"

	// Now stream the data from the sender to the receiver
	buffer := make([]byte, 4096)
	for {
		nr, errRead := sender.Read(buffer)
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

func getMimeType(path string) string {
	splits := strings.Split(path, ".")
	extension := splits[len(splits)-1]
	mime_type := mime.TypeByExtension("." + extension)
	log.Println("Mime for", extension, "is", mime_type)
	if mime_type == "" {
		return "application/octet-stream"
	} else {
		return mime_type
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
		w.w.Header().Add("Content-type", getMimeType(path))
		w.w.WriteHeader(http.StatusAccepted)
		// We send the writer to the transferServer goroutine that is waiting. It will
		// take over the channel conn.
		conn <- w
		// When the transfer is finished (success or error), conn will
		// be closed
		<-conn
	}
}

func main() {
	listen_tcp := flag.String("listen-tcp", "localhost:1234", "Listen string for the TCP socket")
	listen_http := flag.String("listen-http", "localhost:8080", "Listen string for the HTTP server")
	http_prefix := flag.String("http-prefix", "localhost:8080", "Prefix for the http(s) urls")
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

	l, err := net.Listen("tcp4", *listen_tcp)
	if err != nil {
		log.Println("listen error", err.Error())
		return
	}
	log.Println("TCP Receiver server started at", *listen_tcp)

	go func() {
		log.Println("HTTP(s) server started at", *listen_http)
		err := http.ListenAndServe(*listen_http, nil)
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
		go transferServer(fd, channel, id, *http_prefix)
	}
}
