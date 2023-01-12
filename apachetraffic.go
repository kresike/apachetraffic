package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"log/syslog"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var ghost, gport, gpref string
var logger *(log.Logger)
var tm TrafficMap
var hostname string

func main() {
	var err error

	logger, err = syslog.NewLogger(syslog.LOG_INFO|syslog.LOG_DAEMON, 0)
	if err != nil {
		fmt.Println("Failed to open connection to syslog")
	}

	viper.SetDefault("GraphiteAddress", "xgraphite")
	viper.SetDefault("GraphitePort", "2003")
	viper.SetDefault("GraphitePrefix", "apachetraffic")

	viper.SetConfigName("apachetraffic")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/etc/apachetraffic")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		logger.Println(fmt.Errorf("error parsing config file: %s", err))
	}

	ghost = viper.GetString("GraphiteAddress")
	gport = viper.GetString("GraphitePort")
	gpref = viper.GetString("GraphitePrefix")

	logger.Println("Configured graphite address: ", ghost, ":", gport, " prefix ", gpref)

	if hostname, err = get_fqdn(); err != nil {
		logger.Println(fmt.Errorf("cannot get system's FQDN hostname: %s", err))
		hostname, _ = os.Hostname()
	}

	go sendStats()

	scanner := bufio.NewScanner(os.Stdin)

	logger.Println("Ready to process traffic information.")

	for scanner.Scan() {
		text := scanner.Text()
		elems := strings.Fields(text)
		vhost := elems[1]
		rbytes, _ := strconv.Atoi(elems[2])
		sbytes, _ := strconv.Atoi(elems[3])
		ttfb, _ := strconv.Atoi(elems[4])
		handler := ""
		if len(elems) > 4 {
			handler = elems[5]
			// if the handler was unknown or not configured properly, it's better to leave it empty
			if handler[0:1] == "-" || handler[0:1] == "[" {
				handler = ""
			}
		}
		now := time.Now().Truncate(time.Minute)
		tl, err := tm.Get(now)
		if err != nil {
			tl = NewTrafficList()
			tm.Add(now, tl)
		}
		tl.AddEntry(vhost, rbytes, sbytes, ttfb, handler)
	}
	if err := scanner.Err(); err != nil {
		logger.Println(fmt.Errorf("error reading from stdin: %s", err))
		panic(fmt.Errorf("error reading from stdin: %s", err))
	}

}

func sendStats() {
	for {
		time.Sleep(60 * time.Second)

		conn, err := net.Dial("tcp", ghost+":"+gport)
		if err != nil {
			logger.Println("Failed to connect to Graphite:", err)
		} else {
			logger.Println("Connected to:", ghost, ":", gport, "Sending traffic information...")

			tm.SendTraffic(conn, gpref, hostname)

			conn.Close()
		}
	}
}

func get_fqdn() (string, error) {
	cmd := exec.Command("/bin/hostname", "-f")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error getting FQDN: %v", err)
	}
	fqdn := out.String()
	fqdn = fqdn[:len(fqdn)-1]

	return fqdn, nil
}
