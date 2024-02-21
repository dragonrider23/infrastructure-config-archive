package grabber

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lfkeitel/inca/src/common"
	"github.com/lfkeitel/verbose"
)

var appLogger *verbose.Logger
var stdOutLogger *verbose.Logger
var configGrabRunning = false
var conf *common.Config

var totalDevices = 0
var finishedDevices = 0
var stage = ""

func Init(config *common.Config) {
	conf = config

	fileLogger, err := verbose.NewFileHandler(filepath.Join(config.Paths.LogDir, "main.log"))
	if err != nil {
		panic("Failed to open logging directory")
	}

	if config.Debug {
		fileLogger.SetMinLevel(verbose.LogLevelDebug)
	} else {
		fileLogger.SetMinLevel(verbose.LogLevelInfo)
	}

	appLogger = verbose.New("grabber")
	appLogger.AddHandler("file", fileLogger)
	appLogger.AddHandler("stdout", verbose.NewStdoutHandler(true))

	stdOutLogger = verbose.New("execStdOut")
	stdOutLogger.AddHandler("file", fileLogger)
	stdOutLogger.AddHandler("stdout", verbose.NewStdoutHandler(true))
}

func PerformConfigGrab() {
	runGeneric(nil)
}

func PerformSingleRun(name, hostname, brand, method string) {
	name = strings.Replace(name, "-", "_", -1)
	hosts := make([]host, 1)
	hosts[0] = host{
		Name:    name,
		Address: hostname,
		Dtype:   brand,
		Method:  method,
	}
	runGeneric(hosts)
}

func runGeneric(hosts []host) {
	if configGrabRunning {
		appLogger.Error("Job already running")
		return
	}

	startTime := time.Now()
	configGrabRunning = true
	defer func() {
		configGrabRunning = false
		stage = ""
	}()

	if conf.Hooks.PreScript != "" {
		appLogger.Info("Running pre script")
		stage = "pre-script"
		if err := exec.Command(conf.Hooks.PreScript).Run(); err != nil {
			appLogger.Error(err)
		}
	}

	stage = "loading-configuration"
	if hosts == nil {
		var err error
		hosts, err = loadDeviceList(conf)
		if err != nil {
			appLogger.Error(err.Error())
			return
		}
	}

	dtypes, err := loadDeviceTypes(conf)
	if err != nil {
		appLogger.Error(err.Error())
		return
	}

	totalDevices = len(hosts)
	finishedDevices = 0
	dateSuffix := time.Now().Format("2006-01-02T15:04:05")

	stage = "grabbing"
	grabConfigs(hosts, dtypes, dateSuffix, conf)

	stage = "cleanup"
	cleanUpHostDirs(hosts)

	if conf.Hooks.PostScript != "" {
		appLogger.Info("Running post script")
		stage = "post-script"
		if err := exec.Command(conf.Hooks.PostScript).Run(); err != nil {
			appLogger.Error(err)
		}
	}

	endTime := time.Now()
	logText := fmt.Sprintf("Config grab took %s", endTime.Sub(startTime).String())
	appLogger.Info(logText)
	common.UserLogInfo(logText)
}

func IsRunning() bool {
	return configGrabRunning
}

func remainingDeviceCount() (total, finished int) {
	if !configGrabRunning {
		if totalDevices == 0 {
			hosts, err := loadDeviceList(conf)
			if err != nil {
				appLogger.Error(err.Error())
				return
			}
			totalDevices = len(hosts)
		}

		if finishedDevices == 0 {
			finishedDevices = totalDevices
		}
	}

	return totalDevices, finishedDevices
}

type State struct {
	Running         bool
	Total, Finished int
	Stage           string
}

func CurrentState() State {
	total, finished := remainingDeviceCount()

	return State{
		Running:  configGrabRunning,
		Total:    total,
		Finished: finished,
		Stage:    stage,
	}
}
