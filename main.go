package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hpcloud/tail"
	"github.com/ilyakaznacheev/cleanenv"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	LogLevel           string `yaml:"LogLevel" toml:"loglevel" env:"LOG_LEVEL"`
	NameSyslogFileName string `yaml:"SyslogFile" toml:"syslog" env:"SYSLOG_FILE"`
}
type request struct {
	Time,
	IP string
	timeInt int
}

type lineInMap struct {
	month,
	day,
	time,
	parent,
	info,
	iface,
	method,
	ip,
	direct,
	mac string
}

type lineOfLog struct {
	time,
	ip,
	mac string
	timeInt int
}

type transport struct {
	mapTable map[string][]lineOfLog
	GMT      string
	mux      sync.Mutex
}

var (
	data *transport = new(transport)
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

/*
Jun 22 21:39:13 192.168.65.1 dhcp,info dhcp_lan deassigned 192.168.65.149 from 04:D3:B5:FC:E8:09
Jun 22 21:40:16 192.168.65.1 dhcp,info dhcp_lan assigned 192.168.65.202 to E8:6F:38:88:92:29
*/

func (data *transport) parseLineLog(lineIn string) (lineOfLog, error) {
	var lineOfLog lineOfLog
	var lineInMap lineInMap
	if !strings.Contains(lineIn, "assigned") {
		return lineOfLog, fmt.Errorf("This is not assigned/deassigned line:%v", lineIn)
	}
	lineInSlice := strings.Split(lineIn, " ")
	if len(lineInSlice) < 10 {
		return lineOfLog, fmt.Errorf("This is not assigned/deassigned line. Too little data:%v", lineIn)
	}
	lineInMap.month = lineInSlice[0]  // Jun
	lineInMap.day = lineInSlice[1]    // 22
	lineInMap.time = lineInSlice[2]   // 21:39:13
	lineInMap.parent = lineInSlice[3] // 192.168.65.1
	lineInMap.info = lineInSlice[4]   // dhcp,info
	lineInMap.iface = lineInSlice[5]  // dhcp_lan
	lineInMap.method = lineInSlice[6] // deassigned or assigned
	lineInMap.ip = lineInSlice[7]     // 192.168.65.149
	lineInMap.direct = lineInSlice[8] // from or to
	lineInMap.mac = lineInSlice[9]    // 04:D3:B5:FC:E8:09

	time, err := data.parseUnixStampStr(&lineInMap)
	if err != nil {
		log.Errorf("Failed to parse datetime(Str) %v", err)
		return lineOfLog, err
	}
	timeInt, err := data.parseUnixStampInt(&lineInMap)
	if err != nil {
		log.Errorf("Failed to parse datetime(Int) %v", err)
		return lineOfLog, err
	}
	lineOfLog.time = time
	lineOfLog.timeInt = timeInt
	lineOfLog.ip = lineInMap.ip
	lineOfLog.mac = lineInMap.mac
	return lineOfLog, nil
}

func (data *transport) parseUnixStamp(lineInMap *lineInMap) (int64, error) {
	year := time.Now().Format("2006") // Only current Year
	datestr := fmt.Sprintf("%v %v %v %v %v", year, lineInMap.month, lineInMap.day, lineInMap.time, data.GMT)
	date, err := time.Parse("2006 Jan _2 15:04:05 -0700", datestr)
	if err != nil {
		return 0, err
	}
	UnixStamp := date.Unix()
	return UnixStamp, nil
}

func (data *transport) parseUnixStampStr(lineInMap *lineInMap) (string, error) {
	UnixStamp, err := data.parseUnixStamp(lineInMap)
	return fmt.Sprint(UnixStamp), err

}

func (data *transport) parseUnixStampInt(lineInMap *lineInMap) (int, error) {
	UnixStamp, err := data.parseUnixStamp(lineInMap)
	return int(UnixStamp), err
}

func (data *transport) getDataFromSyslog(t *tail.Tail) {
	// var lineOfLog lineOfLog
	for {
		for line := range t.Lines {
			lineOfLog, err := data.parseLineLog(line.Text)
			if err != nil {
				log.Debugf("%v", err)
				continue
			}

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

	t, err := tail.TailFile(cfg.NameSyslogFileName, tail.Config{Follow: true})
	if err != nil {
		log.Errorf("Error open Syslog file:%v", err)
	}
	data.GMT = "+0500"
	data.mapTable = make(map[string][]lineOfLog)
	go data.getDataFromSyslog(t)

	go func() {
		<-exitChan
		// HERE Insert commands to be executed before the program terminates
		// writer.Flush()
		log.Debugln("Attempt to shutdown")
		t.Cleanup()
		log.Debugln("Removes inotify watches ")
		t.Stop()
		log.Debugln("Stops the tailing activity")
		NameSyslogFile.Close()
		log.Debugln("Close the open file")
		log.Println("Shutting down")
		os.Exit(0)

	}()

	for {
		fmt.Scan(&request.Time, &request.IP)
		s := data.GetMac(&request)
		fmt.Println(s)
		log.Debugf(" | Request:'%v','%v' response:'%v'", request.Time, &request.IP, s)
	}

}
