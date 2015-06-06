// Copyright 2014 JustAdam (adambell7@gmail.com).  All rights reserved.
// License: MIT

// Docker log file client for logentries.
// All containers need to log to stdout/stderr.  Don't forget logrotation.
package main

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"os"
	"strings"
)

const (
	// Only SSL connections are supported
	defaultLogEntriesHost string = "data.logentries.com:20000"
	//
	defaultDockerHost string = "unix:///var/run/docker.sock"
	//
	defaultCertsPemFile string = "certs.pem"
)

var (
	// Logentries host and port name to connect to.
	logEntriesHost string
	// DLE's logging level
	outputLogLevel string
	// How to connect to the Docker host
	dockerHost string
	// Use this token for all containers found without DLE_TOKEN
	defaultToken string
	// Location to certificates file
	certsPemFile string
)

type LogWriter struct {
	logline chan []byte
	token   string
}

func (l *LogWriter) Write(p []byte) (n int, err error) {
	token := []byte(l.token)
	output := append(token, p...)
	l.logline <- output
	log.Debug(string(output))
	return len(p), nil
}

type LogWatcher struct {
	docker   *docker.Client
	LogLines chan []byte
}

func (lw *LogWatcher) AddContainer(cid string) {
	c, err := lw.docker.InspectContainer(cid)
	if err != nil {
		log.WithFields(log.Fields{
			"ID": cid,
		}).Warn(err)
		return
	}

	var envExpand = func(key string) string {
		for _, v := range c.Config.Env {
			if strings.HasPrefix(v, key+"=") {
				return strings.SplitN(v, "=", 2)[1]
			}
		}
		return ""
	}

	if os.Expand("$DLE_IGNORE", envExpand) != "" {
		log.WithFields(log.Fields{
			"ID": cid,
		}).Info("Ignoring container")
		return
	}

	token := os.Expand("$DLE_TOKEN", envExpand)
	if token == "" {
		token = defaultToken
	}

	lf := log.Fields{
		"ID":    cid,
		"token": token,
	}

	log.WithFields(lf).Info("Watching container")

	logwriter := &LogWriter{
		logline: lw.LogLines,
		token:   token,
	}

	logopts := docker.LogsOptions{
		Container:    cid,
		OutputStream: logwriter,
		ErrorStream:  logwriter,
		Stdout:       true,
		Stderr:       true,
		Follow:       true,
		Tail:         "0",
		RawTerminal:  true,
	}
	err = lw.docker.Logs(logopts)
	if err != nil {
		fmt.Println("error:", err)
	}

	log.WithFields(lf).Info("Stopped watching container")
}

func (lw *LogWatcher) WatchEvents() {
	events := make(chan *docker.APIEvents)
	if err := lw.docker.AddEventListener(events); err != nil {
		log.Fatal(err)
	}

	log.Info("Watching docker events")

	for {
		select {
		case event := <-events:
			log.Debug("Got event:", event)
			if event.Status == "start" {
				go lw.AddContainer(event.ID)
			}
		}
	}
}

func init() {
	flag.StringVar(&defaultToken, "default-token", os.Getenv("DLE_DEFAULT_TOKEN"), "Default log entries token to use for undefined containers")

	var tmp string

	tmp = os.Getenv("DOCKER_HOST")
	if tmp == "" {
		tmp = defaultDockerHost
	}
	flag.StringVar(&dockerHost, "docker-host", tmp, "How to connect to the docker host")

	tmp = os.Getenv("DLE_LOG_ENTRIES_HOST")
	if tmp == "" {
		tmp = defaultLogEntriesHost
	}
	flag.StringVar(&logEntriesHost, "le-host", tmp, "host:port address to logentries")

	tmp = os.Getenv("DLE_PEM_FILE")
	if tmp == "" {
		tmp = defaultCertsPemFile
	}
	flag.StringVar(&certsPemFile, "pem-file", tmp, "Location to logentries's certificate pem file")

	tmp = os.Getenv("DLE_LOG_LEVEL")
	if tmp == "" {
		tmp = "warn"
	}
	flag.StringVar(&outputLogLevel, "log-level", tmp, "Log level verbosity (debug, info, warn, fatal, panic)")

	level, err := log.ParseLevel(outputLogLevel)
	if err != nil {
		level = log.FatalLevel
	}
	log.SetLevel(level)

	log.SetOutput(os.Stderr)
}

func main() {
	flag.Parse()

	if flag.Arg(0) == "help" {
		flag.Usage()
		os.Exit(0)
	}

	if defaultToken == "" {
		fmt.Println("Required --default-token missing.")
		flag.Usage()
		os.Exit(1)
	}

	if logEntriesHost == "" {
		fmt.Println("Required --le-host missing.")
		flag.Usage()
		os.Exit(1)
	}

	client, err := docker.NewClient(dockerHost)
	if err != nil {
		log.Fatal(err)
	}

	logwatcher := &LogWatcher{
		LogLines: make(chan []byte),
		docker:   client,
	}

	lec, err := NewTLSConnection(logEntriesHost)
	if err != nil {
		log.Fatal("Unable to connect to host:", err)
	}
	defer lec.Close()

	go logwatcher.WatchEvents()

	containers, _ := logwatcher.docker.ListContainers(docker.ListContainersOptions{})
	for _, c := range containers {
		go logwatcher.AddContainer(c.ID)
	}

	for l := range logwatcher.LogLines {
		if _, err := lec.Write(l); err != nil {
			log.Warn("Connection lost, attempting reconnect")
			if err := lec.Connect(); err != nil {
				// @todo: store timestamp, current position so we can resend the logs
				log.Fatal("Unable to reconnect:", err)
			}
			lec.Write(l)
		}
	}
}
