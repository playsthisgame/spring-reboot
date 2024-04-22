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
	var command string
	var args cli.StringSlice
	var include string

	app := &cli.App{
		Name:  "Go Watch Run",
		Usage: "A simple utility written in go to watch a dir and run a command when something changes",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "dir",
				Value:       "/Users/chris/workspace/projects/spacebased/spacebased-api",
				Aliases:     []string{"d"},
				Destination: &dir,
			},
			&cli.StringFlag{
				Name:        "command",
				Value:       "mvn",
				Aliases:     []string{"c"},
				Destination: &command,
			},
			&cli.StringSliceFlag{
				Name:        "args",
				Value:       cli.NewStringSlice("spring-boot:run"),
				Aliases:     []string{"a"},
				Destination: &args,
			},
			&cli.StringFlag{
				Name:        "include",
				Value:       "src",
				Aliases:     []string{"i"},
				Destination: &include,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() > 0 {
				dir = c.Args().First()
			}
			if command == "" {
				fmt.Println("Include a command to run")
			}
			if len(args.Value()) == 0 {
				fmt.Println("Include some args")
			}

			w := watcher.New()

			w.SetMaxEvents(1)
			w.FilterOps(watcher.Move, watcher.Write, watcher.Create, watcher.Remove, watcher.Rename)

			if include != "" {
				r := regexp.MustCompile("^.*" + include + ".*$")
				w.AddFilterHook(watcher.RegexFilterHook(r, true))
			}

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
						cmd := exec.Command(command, args.Value()...)
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
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
