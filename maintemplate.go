package main

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"os"
)

func main() {
	var q Q
	if err := json.NewDecoder(os.Stdin).Decode(&q); err != nil {
		log.Fatal(err)
	}

	t, err := template.ParseFS(templatefs, "*")
	if err != nil {
		log.Fatal(err)
	}
	b := new(bytes.Buffer)
	if err := t.Execute(b, q.Items()); err != nil {
		log.Fatal(err)
	}
	io.Copy(os.Stdout, b)
}
