package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const FOLDER = "job_descriptions"

func makefile(name string, description string) (*string, error) {
	name, err := filepath.Abs(path.Join(FOLDER, name+".txt"))
	if err != nil {
		return nil, err
	}
	fs, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	_, err = fs.Write([]byte(description))

	if err != nil {
		return nil, err
	}
	return &name, nil
}

func callPython(fs string, ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("./run_py.sh %s", fs))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	return err
}

func createHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	company := r.FormValue("company")
	description := r.FormValue("description")

	fmt.Println(description)
	company = strings.ReplaceAll(company, " ", "")

	name, err := makefile(company, description)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	_ = name

	err = nil
	ctx := context.Background()
	err = callPython(*name, ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("done making %s", company)))
}

func main() {
	http.HandleFunc("/create", createHandler)
	// Serve the index.html file at the root URL
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
