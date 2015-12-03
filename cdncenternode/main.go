package main

import (
	"flag"
	"mycdn/toolbox"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/syndtr/goleveldb/leveldb"
)

// types
type MassiveNode struct {
	Ip string
	//	Api               *pub.WebApiStub
	//	Info              *pub.LoginReply
}

type Env struct {
	WebPort  int
	ApiPort  int
	MNPort   int
	Domain   string
	LogLevel string
}

type CDNHttpReqInfo struct {
	HostAddr   []string
	ClientAddr []string
	SourceAddr string
	SourceUrl  string
	SortedUrl  string
}

// variables
var env Env
var db *leveldb.DB
var noredirectResTypes []string = []string{"text/html", "text/css", "application/javascript"}

type IPtoMNMap map[string]*MassiveNode
type UIDtoMNMap map[string]map[string]*MassiveNode
type VNameToSourceName map[string]string

var mapIPToMN IPtoMNMap
var mapUidToMNMap UIDtoMNMap

// use name as uid for simplicity
var mapVNameToSourceName VNameToSourceName
var mapLBRing, mapLBRingFailed map[string]*utils.LBRing

func init() {
	log.SetOutput(os.Stderr)
	var logLevels []string = make([]string, 0, 10)
	logLevels = append(logLevels, log.DebugLevel.String())
	logLevels = append(logLevels, log.InfoLevel.String())
	logLevels = append(logLevels, log.WarnLevel.String())
	logLevels = append(logLevels, log.ErrorLevel.String())
	logLevels = append(logLevels, log.FatalLevel.String())
	logLevels = append(logLevels, log.PanicLevel.String())

	flag.StringVar(&env.Domain, "doname", "n1.leither.cn", "domain name of this host")
	flag.IntVar(&env.WebPort, "web", 80, "web port")
	flag.IntVar(&env.ApiPort, "api", 81, "Rest API port")
	flag.IntVar(&env.MNPort, "mnweb", 80, "web port of massive node")
	flag.StringVar(&env.LogLevel, "log", log.DebugLevel.String(), "log level: "+strings.Join(logLevels, ","))

	flag.Parse()
	lvl, err := log.ParseLevel(env.LogLevel)
	if nil == err {
		log.SetLevel(lvl)
	} else {
		log.SetLevel(log.DebugLevel)
		log.Warn("invalid log level, use default")
	}

	dir := os.Args[0] + "_DB"
	db, err = leveldb.OpenFile(dir, nil)
	if err != nil {
		panic(err)
	}

	mapIPToMN = make(IPtoMNMap)
	mapUidToMNMap = make(UIDtoMNMap)
	mapVNameToSourceName = make(VNameToSourceName)

	//read configs from db
	utils.GobRegister(mapIPToMN, mapUidToMNMap, mapVNameToSourceName)
	if b, err := db.Get([]byte("mapIPToMN"), nil); err == nil {
		if err := utils.GetInterface(b, &mapIPToMN); err == nil {
			log.Infof("success loaded conf %v", mapIPToMN)
		}
	}
	if b, err := db.Get([]byte("mapUidToMNMap"), nil); err == nil {
		if err := utils.GetInterface(b, &mapUidToMNMap); err == nil {
			log.Infof("success loaded conf %v", mapUidToMNMap)
		}
	}
	if b, err := db.Get([]byte("mapVNameToSourceName"), nil); err == nil {
		if err := utils.GetInterface(b, &mapVNameToSourceName); err == nil {
			log.Infof("success loaded conf %v", mapVNameToSourceName)
		}
	}

	mapLBRing = BuildLBRingFromUidMap()
	mapLBRingFailed = make(map[string]*utils.LBRing)

	RefreshLBRing()
}

//
func main() {
	defer db.Close()
	ticker := time.NewTicker(time.Second * 60 * 1)
	go func() {
		for _ = range ticker.C {
			RefreshLBRing()
		}
	}()
	startWebSrv()
	startApiSrv()
}
