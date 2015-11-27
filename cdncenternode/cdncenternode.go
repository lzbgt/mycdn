package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"toolbox"

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
	WebPort int
	ApiPort int
	MNPort  int
	Domain  string
}

// variables
var env Env
var db *leveldb.DB

type IPtoMNMap map[string]*MassiveNode
type UIDtoMNMap map[string]map[string]*MassiveNode
type VNameToSourceName map[string]string

var mapIPToMN IPtoMNMap
var mapUidToMNMap UIDtoMNMap

// use name as uid for simplicity
var mapVNameToSourceName VNameToSourceName

func init() {
	flag.StringVar(&env.Domain, "doname", "n1.leither.cn", "domain name of this host")
	flag.IntVar(&env.WebPort, "web", 80, "web port")
	flag.IntVar(&env.ApiPort, "api", 81, "Rest API port")
	flag.IntVar(&env.MNPort, "mnweb", 80, "web port of massive node")
	flag.Parse()

	var err error
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
			log.Printf("INFO success loaded conf %v", mapIPToMN)
		}
	}
	if b, err := db.Get([]byte("mapUidToMNMap"), nil); err == nil {
		if err := utils.GetInterface(b, &mapUidToMNMap); err == nil {
			log.Printf("INFO success loaded conf %v", mapUidToMNMap)
		}
	}
	if b, err := db.Get([]byte("mapVNameToSourceName"), nil); err == nil {
		if err := utils.GetInterface(b, &mapVNameToSourceName); err == nil {
			log.Printf("INFO success loaded conf %v", mapVNameToSourceName)
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

func RefreshMNs() {

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

		//TODO need improvement for delta save
		if err := db.Put([]byte("mapIPToMN"), utils.GetBytes(&mapIPToMN), nil); err != nil {
			log.Printf("ERR failed to save db: %v", mapIPToMN)
		}
		if err := db.Put([]byte("mapUidToMNMap"), utils.GetBytes(&mapUidToMNMap), nil); err != nil {
			log.Printf("ERR failed to save db: %v", mapUidToMNMap)
		}
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
		log.Printf("INFO bind vname ok: %v->%v, oldval:%v", name, source, q)
		c.String(http.StatusOK, "bind vname ok: %v->%v, oldval:%v", name, source, q)
		//TODO need improvement for delta save
		if err := db.Put([]byte("mapVNameToSourceName"), utils.GetBytes(&mapVNameToSourceName), nil); err != nil {
			log.Printf("ERR failed to save db: %v", mapVNameToSourceName)
		}
	})

	router.GET("/dump", func(c *gin.Context) {
		ret := "mapVNameToSourceName:\n"
		for k, v := range mapVNameToSourceName {
			ret += "\tkey: " + k + ", v: " + v + "\n"
		}

		ret += "mapUidToMNMap:\n"
		for k, v := range mapUidToMNMap {
			for m, n := range v {
				ret += "\t" + "k: " + k + " m:" + m + " ,n:" + n.Ip
			}
		}

		c.String(http.StatusOK, ret)
	})

	router.Run(":" + strconv.Itoa(env.ApiPort))
}

func CheckHeaderContainsAny(h http.Header, tag string, values ...string) bool {
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

func httphandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("clientAddr: %v, Url:%v\n", r.RemoteAddr, r.Host+r.RequestURI)
	// get source
	dname := strings.Split(r.Host, ":")

	sourceAddr, ok := mapVNameToSourceName[dname[0]]
	if !ok {
		log.Printf("no such dynamic name registered: %v\n", dname)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Invalid user of LOSCDN. Please register on http://leither.cn"))
		return
	}

	// additional port exists
	if len(dname) == 2 {
		sourceAddr += ":" + dname[1]
	}

	sourceUrl := "http://" + sourceAddr + r.RequestURI
	sortedUrl := utils.GetParamsSortedUrl(sourceUrl)
	//log.Printf("source url: %v, %v", sourceUrl, sortedUrl)

	var b []byte
	var err error
	if b, err = db.Get([]byte(sortedUrl), nil); err == nil {
		dbEntry := utils.DBEntry{}
		if err = utils.GetInterface(b, &dbEntry); err == nil {
			header := dbEntry["header"].(http.Header)
			//if ok := CheckHeaderContainsAny(header, "Content-Type", "text/html", "application/javascript"); ok {
			for k, v := range header {
				w.Header().Set(k, strings.Join(v, ""))
			}
			w.Header().Del("Date")
			w.Write(dbEntry["body"].([]byte))
			log.Printf("INFO success response with local cache %#v", sortedUrl)
			return
			//}
		}
	}

	// we don't redirect html page, because it will leave our cdn center node
	// so check head first
	//if b, err := db.Get()
	if resp, err := http.Head(sortedUrl); err != nil {
		log.Printf("ERR failed request header for %v", sourceUrl)
		fmt.Fprintf(w, "ERR failed request header for %v", sourceUrl)
	} else {
		// check head, respond visible page directly
		// "Content-Type":[]string{"text/html"}
		if ok := CheckHeaderContainsAny(resp.Header, "Content-Type", "text/html", "text/css", "application/javascript"); ok {
			// cache it
			dbChan := make(chan utils.DBEntry, 1)
			go utils.CacheRes(sortedUrl, db, dbChan)
			value := <-dbChan
			for k, v := range value["header"].(http.Header) {
				w.Header().Set(k, strings.Join(v, ""))
			}
			w.Header().Del("Date")
			w.Write(value["body"].([]byte))
			log.Printf("INFO susscess cached to local, type %v, url %v", resp.Header["Content-Type"], sortedUrl)
			return
		}
	}

	// get remote IP
	clientAddr := strings.Split(r.RemoteAddr, ":")
	if mns := GetNearestMNs(dname[0], clientAddr[0]); len(mns) == 0 {
		log.Println("WARN no massive node available for ", sourceUrl, ", use source only")
		// redirect to source
		http.Redirect(w, r, sourceUrl, http.StatusTemporaryRedirect)
	} else {
		var mn *MassiveNode
		for _, v := range mns {
			mn = v
			break
		}

		Url, err := url.Parse("http://" + mn.Ip + ":" + strconv.Itoa(env.MNPort) + "/getres")
		if nil != err {
			log.Printf("ERR %v", err)
			return
		}

		params := url.Values{}
		params.Add("bid", dname[0])
		params.Add("source", sourceUrl)
		Url.RawQuery = params.Encode()
		http.Redirect(w, r, Url.String(), http.StatusTemporaryRedirect)
		log.Printf("INFO redirect to massive node: %v, %v", Url.String(), sourceUrl)
	}
}
