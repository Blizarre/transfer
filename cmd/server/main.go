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
)

func generateID() string {
	return fmt.Sprintf("%v", rand.Uint32()%100000)
}

type Message struct {
	data []byte
	err  error
}

func transferServer(c net.Conn, listener chan io.Writer, url string) {
	fmt.Println("Serving", url, "- waiting for a connection")
	_, err := c.Write([]byte(url + "\n"))
	if err != nil {
		panic("Write error: " + err.Error())
	}

	receiver := <-listener

	buffer := make([]byte, 4096)

	for {
		nr, errRead := c.Read(buffer)
		if errRead != nil && errRead != io.EOF {
			log.Println("Error while reading", errRead.Error())
			return
		}
		log.Println("Read", nr)

		_, err = receiver.Write(buffer[:nr])
		if err != nil {
			log.Println("Error while Writing:", err.Error())
			return
		}

		if errRead == io.EOF {
			log.Println("End of", url)
			listener <- nil
			return
		}
	}

}

func serveFile(mapping *SafeMap, w FlushWriter, path string) {
	log.Println("Serving", path)
	splits := strings.Split(path, "/")
	if len(splits) != 3 {
		log.Println("Error: Invalid path", path)
		return
	}
	id := splits[1]

	conn, ok := mapping.Read(id)
	if !ok {
		w.w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("File not found"))
		return
	}

	w.w.WriteHeader(http.StatusAccepted)
	conn <- w
	<-conn
}

func main() {
	listen_str := flag.String("host-receiver", "localhost:1234", "Definition for the receiver")
	listen_http_str := flag.String("host-sender", "localhost:8080", "Definition for the host")
	flag.Parse()

	mapping := NewSafeMap()

	http.HandleFunc("/", func(writer http.ResponseWriter, r *http.Request) {
		flusher, ok := writer.(http.Flusher)
		if !ok {
			log.Println("Error, un-chunkable")
		}

		w := FlushWriter{
			f: flusher,
			w: writer,
		}

		serveFile(&mapping, w, r.URL.Path)
	})

	go http.ListenAndServe(*listen_http_str, nil)
	log.Println("Sender server started, listening to", *listen_http_str)

	l, err := net.Listen("tcp4", *listen_str)
	if err != nil {
		log.Println("listen error", err.Error())
		return
	}
	log.Println("Receiver server started, listening to", *listen_str)

	for {
		fd, err := l.Accept()
		if err != nil {
			log.Println("accept error", err.Error())
			return
		}
		channel := make(chan io.Writer)
		id := generateID()
		mapping.Add(id, channel)
		url := fmt.Sprintf("http://%v/%v/file.bin", *listen_http_str, id)
		go transferServer(fd, channel, url)
	}
}
