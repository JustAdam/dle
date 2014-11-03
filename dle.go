// Copyright 2014 JustAdam (adambell7@gmail.com).  All rights reserved.
// License: MIT

// Docker log file client for logentries.
// All containers need to log to stdout/stderr.  Don't forget logrotation.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ActiveState/tail"
	"github.com/BurntSushi/toml"
	log "github.com/Sirupsen/logrus"
	"gopkg.in/fsnotify.v1"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

const (
	// Only SSL connections are supported
	defaultLogEntriesHost string = "data.logentries.com:20000"
)

var (
	logEntriesHost string
	// Log to this token for all unknown containers
	defaultLogKey      string
	configFileLocation string
	// Location to Docker's container logs
	logDirectory string
	// Parse the JSON stored in Docker's log files
	parseDockerLogFormat bool
	// Location to Docker containers directory
	watchLogDirectory bool
	// Our own log level
	outputLogLevel string
	// Pattern to find Docker container log files
	LogFilePattern string = "*/*.log"

	tailConfig = tail.Config{
		Follow: true,
		ReOpen: true,
		Location: &tail.SeekInfo{
			Offset: 0,
			Whence: 2,
		},
		Logger: tail.DiscardingLogger,
	}

	watchLogs *WatchLogs
	logLines  chan *LogLine
)

type WatchLogs struct {
	sync.Mutex
	Containers map[string]*LogWatch `toml:"container"`
	Ignore     []string
}

type LogWatch struct {
	Key  string
	File string
	Name string
	Quit chan bool `toml:"-"`
}

type LogLine struct {
	Line *tail.Line
	Key  string
	Name string
}

type DockerLogEntry struct {
	Log    string
	Stream string
	// ignoring time for now
}

func (dl *DockerLogEntry) String() string {
	return fmt.Sprintf("(%s) %s", dl.Stream, dl.Log)
}

func init() {
	var tmp string

	tmp = os.Getenv("DLE_CONFIG_FILE")
	if tmp == "" {
		tmp = "config.toml"
	}
	flag.StringVar(&configFileLocation, "config", tmp, "Location of configuration file")

	flag.StringVar(&defaultLogKey, "default-log-key", os.Getenv("DLE_DEFAULT_LOG_KEY"), "Default log entries key to use for undefined containers")

	flag.StringVar(&logDirectory, "log-directory", os.Getenv("DLE_LOG_DIRECTORY"), "Path to Docker containers directory")

	tmpb, err := strconv.ParseBool(os.Getenv("DLE_WATCH_LOG_DIRECTORY"))
	if err != nil {
		tmpb = false
	}
	flag.BoolVar(&watchLogDirectory, "watch-ld", tmpb, "Watch the Docker containers directory for new/removed containers")

	tmpb, err = strconv.ParseBool(os.Getenv("DLE_PARSE_DOCKER_LOGS"))
	if err != nil {
		tmpb = true
	}
	flag.BoolVar(&parseDockerLogFormat, "parse-docker-logs", tmpb, "Parse the Docker log format (false will send the raw log entry)")

	tmp = os.Getenv("DLE_CONFIG_FILE")
	if tmp == "" {
		tmp = defaultLogEntriesHost
	}
	flag.StringVar(&logEntriesHost, "le-host", tmp, "host:port address to logentries")

	tmp = os.Getenv("DLE_LOG_LEVEL")
	if tmp == "" {
		tmp = "fatal"
	}
	flag.StringVar(&outputLogLevel, "log-level", tmp, "Log level verbosity (debug, info, warn, fatal, panic)")

	level, err := log.ParseLevel(outputLogLevel)
	if err != nil {
		level = log.FatalLevel
	}
	log.SetLevel(level)
}

func main() {
	flag.Parse()

	if flag.Arg(0) == "help" {
		flag.Usage()
		os.Exit(0)
	}

	if defaultLogKey == "" {
		fmt.Println("Required --default-log-key missing.")
		flag.Usage()
		os.Exit(1)
	}

	if logDirectory == "" {
		fmt.Println("Required --log-directory missing.")
		flag.Usage()
		os.Exit(1)
	}

	if logEntriesHost == "" {
		fmt.Println("Required --le-host missing.")
		flag.Usage()
		os.Exit(1)
	}

	// check logDirectory exists - hopefully it is the Docker log directory :)
	if ok, err := isDir(logDirectory); ok == false {
		if err != nil {
			log.Fatal(err)
		}
		log.Fatal(logDirectory, "is not a directory")
	}

	go signalHandler()

	watchLogs = New(configFileLocation)

	lec := &LogEntriesConnection{}
	if err := lec.Connect(logEntriesHost); err != nil {
		log.Fatal("Unable to connect to logentries:", err)
	}
	defer lec.Close()

	logLines = make(chan *LogLine)

	if watchLogDirectory {
		go WatchDockerLogDirectory()
	}

	watchLogs.Start()

	for {
		select {
		case line := <-logLines:
			fmt.Fprintln(lec, line)
		}
	}
}

func New(file string) (watchLog *WatchLogs) {
	watchLog = new(WatchLogs)
	watchLog.Lock()
	if _, err := toml.DecodeFile(file, watchLog); err != nil {
		log.Fatal("Unable to open config file:", err)
	}

	// Find all Docker log files
	files, err := filepath.Glob(filepath.Join(logDirectory, LogFilePattern))
	if err != nil {
		log.Fatal("Unable to open log directory:", err)
	}

	// Match up files against config settings
	for _, file := range files {
		watchLog.AddLog(file)
	}

	// Remove ignored containers
	for _, cid := range watchLog.Ignore {
		if _, ok := watchLog.Containers[cid]; ok {
			delete(watchLog.Containers, cid)
			log.WithFields(log.Fields{
				"container": cid,
			}).Info("Ignoring container")
		}
	}

	// Remove container logs which don't exist.
	// These IDs were specified in the config file, but no container was found
	for k, cl := range watchLog.Containers {
		if cl.File == "" {
			delete(watchLog.Containers, k)
			log.WithFields(log.Fields{
				"container": k,
			}).Warn("Container doesn't exist")
		}
	}

	watchLog.Unlock()
	return
}

func (wl *WatchLogs) AddLog(file string) {

	var logKey string
	var logName string
	var containerID string

	filename := filepath.Base(file)

	for start, c := range wl.Containers {
		if match, _ := filepath.Match(start+"*", filename); match {
			containerID = start
			logKey = c.Key
			logName = c.Name
			break
		}
	}

	// Use full container ID if no match found
	if containerID == "" {
		containerID = filepath.Base(filepath.Dir(file))
	}

	// Log against default key if none found
	if logKey == "" {
		logKey = defaultLogKey
	}

	wl.Containers[containerID] = &LogWatch{
		Key:  logKey,
		Name: logName,
		File: file,
		Quit: make(chan bool),
	}
}

func (wl *WatchLogs) Start() {
	for _, container := range wl.Containers {
		go Watch(logLines, container)
	}
}

func Watch(logline chan<- *LogLine, logfile *LogWatch) {
	if logfile.File == "" {
		log.WithFields(log.Fields{
			"key": logfile.Key,
		}).Warn("Can't watch empty logfile")
		return
	}

	t, err := tail.TailFile(logfile.File, tailConfig)
	if err != nil {
		log.WithFields(log.Fields{
			"file": logfile.File,
		}).Warn("Unable to tail file:", err)
	}

	log.WithFields(log.Fields{
		"name": logfile.Name,
		"file": logfile.File,
	}).Info("Watching file")

	for {
		select {
		case line := <-t.Lines:
			ll := &LogLine{
				Key:  logfile.Key,
				Name: logfile.Name,
				Line: line,
			}
			logline <- ll
		case <-logfile.Quit:
			log.WithFields(log.Fields{
				"name": logfile.Name,
				"file": logfile.File,
			}).Info("Stopping tail")
			t.Stop()
			logfile.Quit <- true
			return
		}
	}
}

func signalHandler() {
	sigDie := make(chan os.Signal, 1)
	signal.Notify(sigDie, syscall.SIGINT, syscall.SIGTERM)
	sigReload := make(chan os.Signal, 1)
	signal.Notify(sigReload, syscall.SIGHUP)

	for {
		select {
		case <-sigDie:
			watchLogs.cleanup()
			os.Exit(0)
		case <-sigReload:
			watchLogs.cleanup()
			watchLogs = nil
			watchLogs = New(configFileLocation)
			watchLogs.Start()
		}
	}
}

func (wl *WatchLogs) cleanup() {
	for _, container := range wl.Containers {
		container.Quit <- true
		<-container.Quit
	}
	tail.Cleanup()
}

func (ll *LogLine) String() string {
	var line string
	if parseDockerLogFormat {
		var dlog DockerLogEntry
		if err := json.Unmarshal([]byte(ll.Line.Text), &dlog); err != nil {
			log.Warn("Error decoding log:", err)
			return ""
		}
		line = dlog.String()
	} else {
		line = ll.Line.Text
	}
	return fmt.Sprintf("%s %s %s", ll.Key, ll.Name, line)
}

func isDir(name string) (bool, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return false, err
	}

	if !fi.IsDir() {
		return false, nil
	}

	return true, nil
}

func WatchDockerLogDirectory() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	watcher.Add(logDirectory)

	log.WithFields(log.Fields{
		"dir": logDirectory,
	}).Info("Watching docker logs directory")

	quit := make(chan bool, 1)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					// @todo refactor - removal of dupe code
					if ok, _ := isDir(event.Name); ok {

						log.WithFields(log.Fields{
							"watch": logDirectory,
						}).Debug("New directory creation detected")

						cid := filepath.Base(event.Name)
						addFile := true

						// check if container is ignored
						for _, icid := range watchLogs.Ignore {
							if cid == icid {
								log.WithFields(log.Fields{
									"container": cid,
								}).Info("Ignoring container")
								addFile = false
								continue
							}
						}

						if addFile {
							file := filepath.Join(event.Name, cid+"-json.log")
							if err != nil {
								log.WithFields(log.Fields{
									"file": file,
									"cid":  cid,
								}).Warn("Error finding log")
							} else {
								watchLogs.Lock()
								watchLogs.AddLog(file)
								watchLogs.Unlock()
								go Watch(logLines, watchLogs.Containers[cid])
							}
						}
					}
				case event.Op&fsnotify.Remove == fsnotify.Remove:

					log.WithFields(log.Fields{
						"watch": logDirectory,
					}).Debug("Removal detected")

					cid := filepath.Base(event.Name)

					if c, ok := watchLogs.Containers[cid]; ok {
						c.Quit <- true
						<-c.Quit
						watchLogs.Lock()
						delete(watchLogs.Containers, cid)
						watchLogs.Unlock()
						log.WithFields(log.Fields{
							"cid": cid,
						}).Debug("Container log watch removed")
					}
				}
			case err := <-watcher.Errors:
				log.WithFields(log.Fields{
					"func": "DockerLogDirectoryWatch",
				}).Warn(err)
			}
		}
	}()

	<-quit
}
