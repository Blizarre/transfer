package main

import "net/http"

type FlushWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func (f FlushWriter) Write(data []byte) (int, error) {
	n, err := f.w.Write(data)
	if err != nil {
		return n, err
	}
	f.f.Flush()
	return n, err
}
