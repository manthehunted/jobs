package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const FOLDER = "job_descriptions"

const OPENCODE = "opencode run \"{prompt}\""

const MAKE_RESUME = "follow tmpl/Prompt_Resume.md and make {company}.json using tmpl/ResumeContents.md and {job_description}"

type Path string

func (p Path) ReplaceWith(name string) Path {
	parts := strings.Split(string(p), "/")
	n := len(parts)
	if n == 0 {
		panic(fmt.Sprintf("parts should >0, %s", name))
	}
	last := parts[n-1]
	name = name + path.Ext(last)
	parts = parts[:n-1]
	parts = append(parts, name)
	return Path(strings.Join(parts, "/"))
}

func makefile(name string, description string) (Path, error) {
	name, err := filepath.Abs(path.Join(FOLDER, name+".txt"))
	var path Path
	if err != nil {
		return path, err
	}
	fs, err := os.Create(name)
	if err != nil {
		return path, err
	}

	_, err = fs.Write([]byte(description))

	if err != nil {
		return path, err
	}
	return Path(name), nil
}

func argOpenCode(fs Path, company string) string {
	prompt := strings.Replace(MAKE_RESUME, "{job_description}", string(fs), -1)
	prompt = strings.Replace(prompt, "{company}", company, -1)
	cmd := strings.Replace(OPENCODE, "{prompt}", prompt, -1)
	fmt.Println(cmd)
	return cmd
}

func runCmd(ctx context.Context, cmdstr string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdstr)
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func watcher(ctx context.Context, cmdstr string, condition func(string) bool) (string, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", cmdstr)
	var err error
	var fs string

	CmdStdout, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	teeReader := io.TeeReader(CmdStdout, os.Stdout)

	err = cmd.Start()
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(teeReader)
	for scanner.Scan() {
		txt := scanner.Text()
		if condition(txt) {
			fs = txt
			fmt.Fprintf(os.Stdout, "caught %s\n", txt)
			break
		}
	}
	if err = scanner.Err(); err != nil {
		fmt.Printf("failed %s\n", err)
	}

	if err = cmd.Wait(); err != nil {
		return "", err
	}

	return fs, nil
}

func parseString(line string) Path {
	parts := strings.Split(line, "Write")
	var fs string
	for _, part := range parts {
		if strings.Contains(part, "json") {
			fs = strings.TrimSpace(part)
		}
	}
	if fs == "" {
		panic(line)
	}
	return Path(fs)
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

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 2*60*time.Second)
	defer cancel()
	fs, err := watcher(ctx, argOpenCode(name, company), func(text string) bool {
		return strings.Contains(text, "Write") && strings.Contains(text, "json")
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	newfs := parseString(fs).ReplaceWith(company)
	fmt.Println(newfs)
	ctx = context.Background()
	ctx, cancel = context.WithTimeout(ctx, 2*60*time.Second)
	defer cancel()
	err = runCmd(ctx, fmt.Sprintf("./run_py.sh %s", newfs))
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
