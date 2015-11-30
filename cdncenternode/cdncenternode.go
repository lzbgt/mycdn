package main

import (
	"flag"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"toolbox"

	log "github.com/Sirupsen/logrus"

	"github.com/gin-gonic/gin"
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
}

// new Massive Node
func NewMassiveNode(uid, ip string) (mn *MassiveNode) {
	mn = &MassiveNode{ip}
	mapIPToMN[ip] = mn
	if uid != "" {
		if _, ok := mapUidToMNMap[uid]; !ok {
			mapUidToMNMap[uid] = make(map[string]*MassiveNode)
		}
		mapUidToMNMap[uid][ip] = mn
	}

	return mn
}

//
func GetNearestMNs(uid, clientIP string) map[string]*MassiveNode {
	if mns, ok := mapUidToMNMap[uid]; ok && len(mns) > 0 {
		return mns
	} else {
		return map[string]*MassiveNode{}
	}
}

//
func saveNodeInfo() {
	//TODO need improvement for delta save
	if err := db.Put([]byte("mapIPToMN"), utils.GetBytes(&mapIPToMN), nil); err != nil {
		log.Errorf("failed to save db: %v", mapIPToMN)
	}
	if err := db.Put([]byte("mapUidToMNMap"), utils.GetBytes(&mapUidToMNMap), nil); err != nil {
		log.Errorf("failed to save db: %v", mapUidToMNMap)
	}
}

//
func saveBindingInfo() {
	//TODO need improvement for delta save
	if err := db.Put([]byte("mapVNameToSourceName"), utils.GetBytes(&mapVNameToSourceName), nil); err != nil {
		log.Errorf("failed to save db: %v", mapVNameToSourceName)
	}
}

//
func main() {
	// web server
	http.HandleFunc("/", httphandler)
	log.Println("INFO start web server on port ", env.WebPort)
	go func() {
		if err := http.ListenAndServe(":"+strconv.Itoa(env.WebPort), nil); nil != err {
			panic(err)
		}
	}()

	// api server
	router := gin.Default()
	router.GET("/addmn", func(c *gin.Context) {
		name := c.Query("name")
		if len(name) == 0 {
			c.String(http.StatusOK, "ERR no name provided")
			return
		}

		ip := c.Query("ip")
		ips := strings.Split(ip, ",")
		if len(ip) == 0 {
			c.String(http.StatusOK, "ERR no ip provided")
			return
		}

		ret := "Nodes added for " + name + ":\n"

		for _, v := range ips {
			if v != "" {
				NewMassiveNode(name, v)
				ret += "\t" + ip + "\n"
			}
		}
		log.Println("INFO success add massive node", ret)
		c.String(http.StatusOK, ret)
		saveNodeInfo()
	})

	router.GET("/bind", func(c *gin.Context) {
		name := c.Query("name")
		source := c.Query("source")
		if len(name) == 0 || len(source) == 0 {
			c.String(http.StatusOK, "ERR no name/source")
			return
		}

		q, _ := mapVNameToSourceName[name]
		mapVNameToSourceName[name] = source
		log.Infof("bind vname ok: %v->%v, oldval:%v", name, source, q)
		c.String(http.StatusOK, "bind vname ok: %v->%v, oldval:%v", name, source, q)
		saveBindingInfo()
	})

	router.GET("/dump", func(c *gin.Context) {
		ret := "mapVNameToSourceName:\n"
		for k, v := range mapVNameToSourceName {
			ret += "\tkey: " + k + ", v: " + v + "\n"
		}

		ret += "mapUidToMNMap:\n"
		for k, v := range mapUidToMNMap {
			for m, n := range v {
				ret += "\t" + "k: " + k + " m:" + m + " ,n:" + n.Ip + "\n"
			}
		}
		c.String(http.StatusOK, ret)
	})

	router.GET("/del", func(c *gin.Context) {
		obj := c.Query("obj")
		key1 := c.Query("key1")
		key2 := c.Query("key2")

		if obj == "vname" && key1 != "" {
			delete(mapVNameToSourceName, key1)
		} else if obj == "node" && key1 != "" {
			if key2 == "" && len(mapUidToMNMap[key1]) == 0 {
				delete(mapUidToMNMap, key1)
			} else if key2 != "" {
				delete(mapUidToMNMap[key1], key2)
			}
		} else {
			c.String(http.StatusOK, "usage: /del?obj=<vname|node>&key1=<>&key2=<>")
			return
		}

		c.String(http.StatusOK, "done")
		saveBindingInfo()
		saveNodeInfo()

	})

	router.Run(":" + strconv.Itoa(env.ApiPort))
}

//
func httphandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("clientAddr: %v, Url:%v\n", r.RemoteAddr, r.Host+r.RequestURI)
	httpReqInfo := getHttpReqInfo(w, r)

	if httpReqInfo.SourceAddr == "" {
		return
	}

	// check cache
	var b []byte
	var err error
	if b, err = db.Get([]byte(httpReqInfo.SortedUrl), nil); err == nil {
		dbEntry := utils.DBEntry{}
		if err = utils.GetInterface(b, &dbEntry); err == nil {
			header := dbEntry["header"].(http.Header)
			// html, css, js
			if ok := CheckHeaderContainsAny(header, "Content-Type", noredirectResTypes); ok {
				for k, v := range header {
					w.Header().Set(k, strings.Join(v, ""))
				}
				w.Header().Del("Date")
				w.Write(dbEntry["body"].([]byte))
				log.Infof("success response with local cache, type: %v, url: %v", header["Content-Type"], httpReqInfo.SortedUrl)
				return
			} else {
				// redirect
				redirectResources(w, r, &httpReqInfo)
				return
			}
		}
	}

	// no cache hitted
	// we don't redirect html page, because it will leave the domain of the cdn center node
	// so check head first
	if resp, err := http.Head(httpReqInfo.SortedUrl); err != nil {
		log.Errorf("failed request header for %v", httpReqInfo.SourceUrl)
	} else {
		// check head, respond visible page directly
		// "Content-Type":[]string{"text/html"}
		if ok := CheckHeaderContainsAny(resp.Header, "Content-Type", noredirectResTypes); ok {
			// cache it
			dbChan := make(chan utils.DBEntry, 1)
			go utils.CacheRes(httpReqInfo.SortedUrl, db, dbChan)
			value := <-dbChan
			for k, v := range value["header"].(http.Header) {
				w.Header().Set(k, strings.Join(v, ""))
			}
			w.Header().Del("Date")
			w.Write(value["body"].([]byte))
			log.Infof("susscess cached to local, type %v, url %v", resp.Header["Content-Type"], httpReqInfo.SortedUrl)

			return
		}
	}

	// below handles resources that are not html, css, js or when source server down
	redirectResources(w, r, &httpReqInfo)
}

func redirectResources(w http.ResponseWriter, r *http.Request, httpReqInfo *CDNHttpReqInfo) {
	if mns := GetNearestMNs(httpReqInfo.HostAddr[0], httpReqInfo.ClientAddr[0]); len(mns) == 0 {
		log.Println("WARN no massive node available for ", httpReqInfo.SourceUrl, ", use source only")
		// redirect to source
		http.Redirect(w, r, httpReqInfo.SourceUrl, http.StatusTemporaryRedirect)
	} else {
		var mn *MassiveNode
		for _, v := range mns {
			mn = v
			break
		}

		//
		Url, err := url.Parse("http://" + mn.Ip + ":" + strconv.Itoa(env.MNPort) + "/getres")
		if nil != err {
			log.Errorf("%v", err)
			return
		}

		//
		params := url.Values{}
		params.Add("bid", httpReqInfo.HostAddr[0])
		params.Add("source", httpReqInfo.SourceUrl)
		Url.RawQuery = params.Encode()
		http.Redirect(w, r, Url.String(), http.StatusTemporaryRedirect)
		log.Infof("redirect to massive node: %v, %v", Url.String(), httpReqInfo.SourceUrl)
	}
}

//
func CheckHeaderContainsAny(h http.Header, tag string, values []string) bool {
	if heads, ok := h[tag]; ok {
		v := strings.Join(heads, "")
		for _, value := range values {
			if strings.Contains(v, value) {
				return true
			}
		}
	}

	return false
}

//
func getHttpReqInfo(w http.ResponseWriter, r *http.Request) CDNHttpReqInfo {
	ret := CDNHttpReqInfo{}
	// get source
	dname := strings.Split(r.Host, ":")
	ret.HostAddr = dname

	sourceAddr, ok := mapVNameToSourceName[dname[0]]
	if !ok {
		ret.SourceAddr = ""
		log.Errorf("no such dynamic name registered: %v\n", dname)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Invalid user of LOSCDN. Please register on http://leither.cn"))
	} else {
		// additional port exists
		if len(dname) == 2 {
			sourceAddr += ":" + dname[1]
		}
		ret.SourceAddr = sourceAddr

		sourceUrl := "http://" + sourceAddr + r.RequestURI
		sortedUrl := utils.GetParamsSortedUrl(sourceUrl)

		ret.SourceUrl = sourceUrl
		ret.SortedUrl = sortedUrl
		ret.ClientAddr = strings.Split(r.RemoteAddr, ":")
	}

	return ret
}
