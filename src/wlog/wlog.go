package wlog

import (
    "glog"
    "flag"
)

func init() {
    flag.Parse()
}

func Info(v ...interface{}) {
    glog.Infoln(v)
}

func Warning(v ...interface{}) {
    glog.Warningln(v)
}

func Error(v ...interface{}) {
    glog.Errorln(v)
}

func Fatal(v ...interface{}) {
    glog.Fatalln(v)
}
