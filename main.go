package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/radovskyb/watcher"
	"github.com/urfave/cli/v2"
)

func main() {
	var dir string
	var port int
	var configFile string
	command := "mvn"
	args := []string{"spring-boot:run"}
	include := "src"

	app := &cli.App{
		Name:  "Spring Reboot",
		Usage: "A simple utility to assist in the development of Spring Boot web apps",
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "A simple utility to assist in the development of Spring Boot web apps",

				Aliases: []string{"s"},
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
					&cli.StringFlag{
						Name:        "configFile",
						Value:       "",
						Aliases:     []string{"cf"},
						Destination: &configFile,
					},
				},
				Action: func(c *cli.Context) error {
					if c.NArg() > 0 {
						dir = c.Args().First()
					}

					startApp(&dir, &command, &args, &configFile)

					w := watcher.New()

					w.SetMaxEvents(1)
					w.FilterOps(watcher.Move, watcher.Write, watcher.Create, watcher.Remove, watcher.Rename)

					r := regexp.MustCompile("^.*" + include + ".*$")
					w.AddFilterHook(watcher.RegexFilterHook(r, true))

					if err := w.AddRecursive(dir); err != nil {
						log.Fatalln(err)
					}

					go func() {
						for {
							select {
							case event := <-w.Event:
								fmt.Println(event)
								stopApp(&port)
								startApp(&dir, &command, &args, &configFile)
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
			},
			{
				Name:    "kill",
				Aliases: []string{"k"},
				Usage:   "Kill a running app on a given port",
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:        "port",
						Value:       8080,
						Aliases:     []string{"p"},
						Destination: &port,
					},
				},
				Action: func(cCtx *cli.Context) error {
					if port == 0 {
						fmt.Println("Please enter a valid port")
					}
					stopApp(&port)
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

func startApp(dir *string, command *string, args *[]string, configFile *string) *os.Process {
	var stdoutBuf, stderrBuff bytes.Buffer
	if len(*configFile) > 0 {
		*args = append(*args, fmt.Sprintf("-Dspring-boot.run.jvmArguments=\"-Dspring.config.additional-location=%v\"", *configFile))
	}
	cmd := exec.Command(*command, *args...)
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuff)
	cmd.Dir = *dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	err := cmd.Start()
	if err != nil {
		fmt.Printf(err.Error())
	}
	return cmd.Process
}

func stopApp(port *int) {
	find, b := exec.Command("lsof", "-n", fmt.Sprintf("-i4TCP:%v", *port)), new(bytes.Buffer)
	find.Stdout = b
	find.Run()
	s := bufio.NewScanner(b)
	for s.Scan() {
		if strings.Contains(s.Text(), "LISTEN") {
			words := strings.Fields(s.Text())
			exec.Command("kill", "-9", words[1]).Run()
			return
		}
	}

	fmt.Printf("process not found on %v", *port)
}
