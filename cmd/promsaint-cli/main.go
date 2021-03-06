package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	log "github.com/Sirupsen/logrus"
	_ "github.com/engineyard/promsaint/logging"
	"github.com/engineyard/promsaint/models"
)

var (
	hostAlert         = flag.Bool("hostalert", false, "This is a host alert")
	serviceAlert      = flag.Bool("servicealert", false, "This is a service alert")
	notify            = flag.String("notify", "blackhole", "Value of notify label")
	notificationType  = flag.String("ntype", "", "PROBLEM / ACKNOWLEDGEMENT / RECOVERY")
	state             = flag.String("state", "", "# Host states: #  UP / DOWN # Service states: #  CRITICAL / WARNING / UNKNOWN / OK")
	host              = flag.String("host", "", "Hostname of firing alert")
	service           = flag.String("service", "", "Servicename of firing alert")
	alertTypeOverride = flag.String("alert-type", "", "Alternative alert type")
	alertName         = flag.String("alert-name", "", "Name of firing alert, when type is not service or host")
	message           = flag.String("msg", "", "Service Output")
	note              = flag.String("note", "", "Service note (Reference link)")
	promsaintUrl      = flag.String("promsaint", "http://localhost:8080", "Url of running promsaint Daemon to post to")
	logFile           = flag.String("log.file", "", "Log all info to file")
	versionFlag       = flag.Bool("version", false, "Print version information")
	firePeriod        = flag.Duration("fire-period", 0, "The period in which the alert fires")
	regex2xx          = regexp.MustCompile(`^2..`)
	Version           string
	BuildTime         string
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version: %s\n", Version)
		fmt.Printf("Built:   %s\n", BuildTime)
		os.Exit(0)
	}

	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			log.Panic(err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFormatter(&log.TextFormatter{})
	}

	// Create a custom logger with the provided command line arguments included
	logger := log.WithFields(log.Fields{
		"args": strings.Join(os.Args, " "),
	})

	// Validate flags
	if !*hostAlert && !*serviceAlert && *alertTypeOverride == "" {
		logger.Fatal("One of -hostalert or -servicealert or -alert-type must be set")
	}

	if (*hostAlert || *serviceAlert) && *alertTypeOverride != "" {
		logger.Warn("If one of -hostalert or -servicealert is set, -alert-type is ignored")
	}

	if *alertTypeOverride != "" && *alertName == "" {
		logger.Fatal("When -alert-type is set, -alert-name needs to be set as well")
	}

	if *hostAlert && *serviceAlert {
		logger.Fatal("Only one of -hostalert or -servicealert can be set")
	}

	alertType := *alertTypeOverride
	if *hostAlert {
		alertType = "host"
	} else if *serviceAlert {
		alertType = "service"
	}

	alert := models.Alert{
		Type:             alertType,
		Notify:           strings.ToLower(*notify),
		NotificationType: *notificationType,
		State:            *state,
		Host:             *host,
		Service:          strings.Replace(*service, " ", "_", -1),
		AlertName:        *alertName,
		Message:          *message,
		Note:             *note,
		FirePeriod:       (*firePeriod).String(),
	}

	buff := new(bytes.Buffer)
	err := json.NewEncoder(buff).Encode(alert)
	if err != nil {
		log.Panic(err)
	}

	logger.Info("Forwarding to Promsaint")
	u, err := url.Parse(*promsaintUrl)
	if err != nil {
		log.Fatal(err)
	}

	u.Path = path.Join(u.Path, "json")
	res, err := http.Post(u.String(), "application/json", buff)
	if err != nil {
		log.Panic(err)
	}
	defer res.Body.Close()

	status := fmt.Sprintf("%d", res.StatusCode)
	logger.Debugf("Status: %s", status)
	if !regex2xx.Match([]byte(status)) {
		logger.Errorf("Promsaint responded with non 2xx error: %s", res.Status)
		body, _ := ioutil.ReadAll(res.Body)
		logger.Debugf("Promsaint response:%s", string(body))
	}
}
