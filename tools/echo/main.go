package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func jsonHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"slideshow": map[string]interface{}{
			"author": "Yours Truly",
			"date":   "date of publication",
			"slides": []map[string]interface{}{
				{"title": "Wake up to WonderWidgets!", "type": "all"},
				{"items": []string{"Why <em>WonderWidgets</em> are great", "Who <em>buys</em> WonderWidgets"}, "title": "Overview", "type": "all"},
			},
			"title": "Sample Slide Show",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	prettyJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	fmt.Println(string(prettyJSON))
	fmt.Print("\n")
	json.NewEncoder(w).Encode(response)
}

func headersHandler(w http.ResponseWriter, r *http.Request) {
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}
	response := map[string]interface{}{
		"headers": headers,
	}
	w.Header().Set("Content-Type", "application/json")
	prettyJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	fmt.Println(string(prettyJSON))
	fmt.Print("\n")
	json.NewEncoder(w).Encode(response)
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	// Parse query params
	query := r.URL.Query()
	args := make(map[string]string)
	for k, v := range query {
		if len(v) > 0 {
			args[k] = v[0]
		}
	}

	// Get IP
	ip := r.RemoteAddr
	if idx := strings.Index(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	// Headers
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	// URL
	url := "http://127.0.0.1:9000" + r.URL.String()

	// For POST, try to parse JSON
	var jsonData interface{}
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			json.Unmarshal(body, &jsonData)
		}
	}

	response := map[string]interface{}{
		"args":    args,
		"headers": headers,
		"origin":  ip,
		"url":     url,
	}

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		response["data"] = string(body)
		response["files"] = map[string]interface{}{}
		response["form"] = map[string]interface{}{}
		if jsonData != nil {
			response["json"] = jsonData
		}
	}

	w.Header().Set("Content-Type", "application/json")
	prettyJSON, _ := json.MarshalIndent(response, "", "  ")
	fmt.Printf("%s %s\n", r.Method, r.URL.Path)
	fmt.Println(string(prettyJSON))
	fmt.Print("\n")
	json.NewEncoder(w).Encode(response)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	// Path like /status/200
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) >= 3 {
		statusStr := parts[2]
		if status, err := strconv.Atoi(statusStr); err == nil {
			w.WriteHeader(status)
			return
		}
	}
	w.WriteHeader(404)
}

func main() {
	http.HandleFunc("/json", jsonHandler)
	http.HandleFunc("/headers", headersHandler)
	http.HandleFunc("/status/", statusHandler)
	http.HandleFunc("/", echoHandler)
	fmt.Println("Echo server running on :9000")
	http.ListenAndServe(":9000", nil)
}
