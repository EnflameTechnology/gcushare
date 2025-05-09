// Copyright (c) 2024, ENFLAME INC.  All rights reserved.

package logs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"gcushare-scheduler-plugin/pkg/consts"
)

var LOGPATH = consts.LOGPATH

func init() {
	// set env: 'LOGPATH' to customize log output path, this usually used for testing
	if logPath := os.Getenv("LOGPATH"); logPath != "" {
		LOGPATH = logPath
	}
	// for run binary alone in the host
	if err := os.MkdirAll(filepath.Dir(LOGPATH), 0755); err != nil {
		panic(fmt.Sprintf("Error creating directory %s: %v", filepath.Dir(LOGPATH), err))
	}
}

func Info(format string, a ...interface{}) {
	file, err := os.OpenFile(LOGPATH, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Faild to open info logger file:%s", err.Error()))
	}
	defer func() {
		file.Close()
	}()
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(file)
	_, callerFileName, callerFileline, ok := runtime.Caller(1)
	if !ok {
		panic("Faild to get caller")
	}
	msg := fmt.Sprintf(format, a...)
	log.Printf("%s %s:%d %s", consts.LOGINFO, relativePath(callerFileName), callerFileline, msg)
}

func Error(errOut interface{}, msg ...interface{}) {
	file, err := os.OpenFile(LOGPATH, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Faild to open info logger file:%s", err.Error()))
	}
	defer func() {
		file.Close()
	}()
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(file)
	_, callerFileName, callerFileline, ok := runtime.Caller(1)
	if !ok {
		panic("Faild to get caller")
	}
	desc := ""
	switch realErr := errOut.(type) {
	case error:
		if len(msg) > 0 {
			format, ok := msg[0].(string)
			if !ok {
				panic(fmt.Sprintln("error message must be string type"))
			}
			desc = fmt.Sprintf(format, msg[1:]...)
		}
	case string:
		errOut = fmt.Sprintf(realErr, msg...)
	default:
		panic(fmt.Sprintln("errOut message must be string or error type"))
	}
	if desc != "" {
		desc += ", "
	}
	log.Printf("%s %s:%d %serror: %v\n%s", consts.LOGERROR, relativePath(callerFileName), callerFileline,
		desc, errOut, string(debug.Stack()))
}

func Warn(format string, a ...interface{}) {
	file, err := os.OpenFile(LOGPATH, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Faild to open info logger file:%s", err.Error()))
	}
	defer func() {
		file.Close()
	}()
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(file)
	_, callerFileName, callerFileline, ok := runtime.Caller(1)
	if !ok {
		panic("Faild to get caller")
	}
	msg := fmt.Sprintf(format, a...)
	log.Printf("%s %s:%d %s", consts.LOGWARN, relativePath(callerFileName), callerFileline, msg)
}

func Debug(format string, a ...interface{}) {
	if os.Getenv("LOG_DEBUG") != "true" {
		return
	}
	file, err := os.OpenFile(LOGPATH, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Faild to open info logger file:%s", err.Error()))
	}
	defer func() {
		file.Close()
	}()
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetOutput(file)
	_, callerFileName, callerFileline, ok := runtime.Caller(1)
	if !ok {
		panic("Faild to get caller")
	}
	msg := fmt.Sprintf(format, a...)
	log.Printf("%s %s:%d %s", consts.LOGDEBUG, relativePath(callerFileName), callerFileline, msg)
}

func relativePath(absolutePath string) string {
	list := strings.Split(absolutePath, "/")
	for index, str := range list {
		if strings.HasPrefix(str, consts.COMPONENT_NAME) {
			return strings.Join(list[index:], "/")
		}
	}
	return absolutePath
}
