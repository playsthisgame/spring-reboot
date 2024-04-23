package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"syscall"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/urfave/cli/v2"
)

func main() {
	var dir string
	var port int
	command := "mvn"
	args := []string{"spring-boot:run"}
	include := "src"

	app := &cli.App{
		Name:  "Spring Reboot",
		Usage: "A simple utility to assist in the development of Spring Boot web apps",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "dir",
				Value:       ".",
				Aliases:     []string{"d"},
				Destination: &dir,
			},
			&cli.IntFlag{
				Name:        "port",
				Value:       8080,
				Aliases:     []string{"p"},
				Destination: &port,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() > 0 {
				dir = c.Args().First()
			}

			w := watcher.New()

			w.SetMaxEvents(1)
			w.FilterOps(watcher.Move, watcher.Write, watcher.Create, watcher.Remove, watcher.Rename)

			r := regexp.MustCompile("^.*" + include + ".*$")
			w.AddFilterHook(watcher.RegexFilterHook(r, true))

			if err := w.AddRecursive(dir); err != nil {
				log.Fatalln(err)
			}

			var stdoutBuf, stderrBuff bytes.Buffer
			var process *os.Process

			go func() {
				for {
					select {
					case event := <-w.Event:
						fmt.Println(event)
						if process != nil {
							// kill process if there is still one running
							pgid, err := syscall.Getpgid(process.Pid)
							if err == nil {
								if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
									log.Fatalln(err)
								}
							}
						}
						cmd := exec.Command(command, args...)
						cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
						cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuff)
						cmd.Dir = dir
						cmd.SysProcAttr = &syscall.SysProcAttr{
							Setpgid: true,
						}

						err := cmd.Start()
						process = cmd.Process
						if err != nil {
							fmt.Printf(err.Error())
						}
					case err := <-w.Error:
						log.Fatalln(err)
					case <-w.Closed:
						return
					}
				}
			}()

			if err := w.Start(time.Millisecond * 100); err != nil {
				log.Fatalln(err)
			}

			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "stop",
				Aliases: []string{"s"},
				Usage:   "Stop a running app on a given port",
				Action: func(cCtx *cli.Context) error {
					if port == 0 {
						fmt.Println("Please enter a valid port")
					}
					out, err := exec.Command(fmt.Sprintf("lsof -i :%v", port)).Output()

					if err != nil {
						fmt.Printf(err.Error())
					}
					exec.Command("kill", "-9", string(out))
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
