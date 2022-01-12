# Transfer

## Disclaimer
**This is an experiment. It is not meant for production use.**

I wanted to learn a bit about channels and coroutines, so this code is really was made just for fun. It's probably only me second go software in 5 years.

## Description

Simple code to expose data from a TCP socket via a http endpoint.

- A client `sender` (`nc` in the examples) creates a connection to the server
- The server send back a URL to the sender and block.
- When another client (the `receiver`) GET the url, the server will stream the data from the `sender` to the `receiver`.

The server will only store the data long enough to transmit it.

Error handling is lacking as there is no way for the server to know if the `sender` failed.


## Example

### Server

```
$ go run ./cmd/server/
2022/01/12 22:52:38 TCP Receiver server started at localhost:1234
2022/01/12 22:52:38 HTTP(s) server started at localhost:8080
2022/01/12 22:52:42 96162 to Sender: http://localhost:8080/96162/file
2022/01/12 22:52:42 96162 Serving http://localhost:8080/96162/file - waiting for a connection
2022/01/12 22:52:45 Serving /96162/file
2022/01/12 22:52:45 Mime for /96162/file is
2022/01/12 22:52:45 96162 to Sender: Starting transfer
2022/01/12 22:52:45 96162 Read 2
2022/01/12 22:52:47 96162 Read 2
2022/01/12 22:52:52 96162 Read 2
2022/01/12 22:52:52 96162 Read 0
2022/01/12 22:52:52 96162 Transfer Successful
2022/01/12 22:52:52 96162 to Sender: Closing
```

### Sender
```
$ (echo A; sleep 5; echo B; sleep 5; echo C) | nc -N localhost 1234
http://localhost:8080/96162/file
Starting transfer
Closing
```

### Receiver
```
$ curl -v http://localhost:8080/96162/file
*   Trying 127.0.0.1:8080...
* Connected to localhost (127.0.0.1) port 8080 (#0)
> GET /96162/file HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.80.0
> Accept: */*
>
* Mark bundle as not supporting multiuse
< HTTP/1.1 202 Accepted
< Content-Type: application/octet-stream
< Date: Wed, 12 Jan 2022 22:52:29 GMT
< Transfer-Encoding: chunked
<
A
B
C
* Connection #0 to host localhost left intact
~ $ curl -v http://localhost:8080/96162/file
*   Trying 127.0.0.1:8080...
* Connected to localhost (127.0.0.1) port 8080 (#0)
> GET /96162/file HTTP/1.1
> Host: localhost:8080
> User-Agent: curl/7.80.0
> Accept: */*
>
* Mark bundle as not supporting multiuse
< HTTP/1.1 202 Accepted
< Content-Type: application/octet-stream
< Date: Wed, 12 Jan 2022 22:52:45 GMT
< Transfer-Encoding: chunked
<
A
B
C
* Connection #0 to host localhost left intact
```
```