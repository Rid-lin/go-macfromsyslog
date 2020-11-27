package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/ilyakaznacheev/cleanenv"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	LogLevel           string `yaml:"LogLevel" toml:"loglevel" env:"LOG_LEVEL"`
	NameSyslogFileName string `yaml:"SyslogFile" toml:"syslog" env:"SYSLOG_FILE"`
}
type request struct {
	Time    string
	IP      string
	timeInt int
}

type lineOfLog struct {
	time    string
	ip      string
	mac     string
	timeInt int
}

type transport struct {
	mapTable map[string][]lineOfLog
	mux      sync.Mutex
}

var (
	data *transport
	cfg  Config
	// writer         *bufio.Writer
	NameSyslogFile *os.File
	// err            error
	configFilename string = "config.toml" //need change
)

func init() {
	flag.StringVar(&cfg.LogLevel, "loglevel", "info", "Log level")
	flag.StringVar(&cfg.NameSyslogFileName, "syslog", "syslog.log", "The file where logs will be written in the format of squid logs")
	flag.Parse()
	var config_source string
	if cfg.NameSyslogFileName == "" {
		err := cleanenv.ReadConfig(configFilename, &cfg)
		if err != nil {
			log.Warningf("No config file(%v) found: %v", configFilename, err)
		}
		lvl, err2 := log.ParseLevel(cfg.LogLevel)
		if err2 != nil {
			log.Errorf("Error in determining the level of logs (%v). Installed by default = Info", cfg.LogLevel)
			lvl, _ = log.ParseLevel("info")
		}
		log.SetLevel(lvl)
		config_source = "ENV/CFG"
	} else {
		config_source = "CLI"
	}

	log.Debugf("Config read from %s: NameSyslogFile=(%v)",
		config_source,
		cfg.NameSyslogFileName)

}

// getExitSignalsChannel Intercepts program termination signals
func getExitSignalsChannel() chan os.Signal {

	c := make(chan os.Signal, 1)
	signal.Notify(c,
		// https://www.gnu.org/software/libc/manual/html_node/Termination-Signals.html
		syscall.SIGTERM, // "the normal way to politely ask a program to terminate"
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGQUIT, // Ctrl-\
		// syscall.SIGKILL, // "always fatal", "SIGKILL and SIGSTOP may not be caught by a program"
		syscall.SIGHUP, // "terminal is disconnected"
	)
	return c

}

func (data *transport) GetMac(request *request) string {
	var response, minTime string

	timeInt, err := strconv.Atoi(request.IP)
	if err != nil {
		return "00:00:00:00:00:00"
	}
	request.timeInt = timeInt

	data.mux.Lock()
	defer data.mux.Unlock()
	timeDB := data.mapTable[request.IP]
	for _, lineOfLog := range timeDB {
		if request.Time > lineOfLog.time && lineOfLog.time > minTime {
			minTime = lineOfLog.time
			response = lineOfLog.mac
		}
	}

	return response
}

func parseLineLog(line string) lineOfLog {
	var lineOfLog lineOfLog

	return lineOfLog
}

func (data *transport) getDataFromSyslog(NameSyslogFileName string) {
	var lineOfLog lineOfLog
	t, err := tail.TailFile(NameSyslogFileName, tail.Config{Follow: true})
	if err != nil {
		log.Errorf("Error open Syslog file:%v", err)
	}

	for {
		for line := range t.Lines {
			lineOfLog = parseLineLog(line.Text)

			timeDB := data.mapTable[lineOfLog.ip]
			timeDB = append(timeDB, lineOfLog)

			data.mux.Lock()
			// defer data.mux.Unlock()
			data.mapTable[lineOfLog.ip] = timeDB
			data.mux.Unlock()
		}
	}
}

func main() {

	/*Creating a channel to intercept the program end signal*/
	exitChan := getExitSignalsChannel()
	var request request

	go func() {
		<-exitChan
		// HERE Insert commands to be executed before the program terminates
		// writer.Flush()
		NameSyslogFile.Close()
		log.Println("Shutting down")
		os.Exit(0)

	}()

	go data.getDataFromSyslog(cfg.NameSyslogFileName)

	for {
		fmt.Scan(&request.Time, &request.IP)
		s := data.GetMac(&request)
		fmt.Println(s)
		log.Debugf(" | Request:'%v','%v' response:'%v'", request.Time, &request.IP, s)
	}

}
