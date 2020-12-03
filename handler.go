package main

import (
	"fmt"
	"net/http"
)

func (s *server) handleIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w,
			`<html>
			<head>
			<title>go-macfromsyslog</title>
			</head>
			<body>
			Более подробно на https://github.com/Rid-lin/go-macfromsyslog
			</body>
			</html>
			`)
	}
}

func (s *server) getmac() http.HandlerFunc {
	var request request
	return func(w http.ResponseWriter, r *http.Request) {
		request.Time = r.URL.Query().Get("time")
		request.IP = r.URL.Query().Get("ip")
		fmt.Fprint(w, s.store.GetMac(&request))
	}
}
