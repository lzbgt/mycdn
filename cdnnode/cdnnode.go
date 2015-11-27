package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"toolbox"

	"github.com/gin-gonic/gin"
	"github.com/hprose/hprose-go"
	"github.com/syndtr/goleveldb/leveldb"
)

type DService struct {
}

type Peer struct {
	IP net.IP
}

var mapPeers map[string]Peer = map[string]Peer{}

// just simply return all peers
func (DService) GetFastNodes(uid, clientIP string) []string {
	keys := make([]string, len(mapPeers))
	i := 0
	for k := range mapPeers {
		keys[i] = k
		i++
	}

	return keys
}

//
type EnvStruct struct {
	WebPort int
	RPCPort int
	HostIP  string
	DBPath  string
}

type DBReader struct {
	*leveldb.DB
	key []byte
}

type DBWriter struct {
	*leveldb.DB
	key []byte
}

//
func (db DBWriter) SetKey(key []byte) {
	db.key = key
}

//
func (db DBWriter) getWriter(key []byte) func(p []byte) (n int, err error) {
	return func(p []byte) (n int, err error) {
		if err := db.Put(key, p, nil); nil != err {
			return 0, err
		} else {
			return len(p), nil
		}
	}
}

func (db DBWriter) Write(p []byte) (n int, err error) {
	return db.getWriter(db.key)(p)
}

// TODO not work
func (db DBReader) GetReader(key []byte) func(p []byte) (n int, err error) {
	return func(p []byte) (n int, err error) {
		return 0, nil
	}
}

var env EnvStruct
var db *leveldb.DB

func init() {
	flag.IntVar(&env.RPCPort, "rpc", 2048, "rpc port")
	flag.IntVar(&env.WebPort, "web", 80, "web port")
	flag.StringVar(&env.HostIP, "host", "127.0.0.1", "host IP")
	flag.StringVar(&env.DBPath, "db", "", "db path")
	flag.Parse()

	if env.DBPath == "" {
		env.DBPath = filepath.Dir(os.Args[0]) + "/DB"
	}
	os.MkdirAll(env.DBPath, 0777)
	var err error
	db, err = leveldb.OpenFile(env.DBPath, nil)
	if err != nil {
		panic(err)
	}
}

func main() {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		db.Close()
		syscall.Exit(0)
	}()

	service := hprose.NewHttpService()
	service.AddMethods(DService{})
	go http.ListenAndServe(":"+strconv.Itoa(env.RPCPort), service)

	// web server
	router := gin.Default()
	router.GET("/getres", func(c *gin.Context) {
		uid := c.Query("bid")
		source := c.Query("source") // shortcut for c.Request.URL.Query().Get("lastname")
		refresh := c.Query("refresh")

		if "" == uid || "" == source {
			Url, _ := url.Parse("http://127.0.0.1:" + strconv.Itoa(env.WebPort) +
				"/getres")
			params := url.Values{}
			params.Add("bid", "_OSYOudPLq5m7Z3S_RcB-QS5Uq3dEwOm7hY9VFSXvoo")
			params.Add("source", "http://121.43.154.122:8866/getres?bid=_OSYOudPLq5m7Z3S_RcB-QS5Uq3dEwOm7hY9VFSXvoo&key=K5_T84l1VQ6gl_Q298TK3kAeBDH5Gn_zsiTAlHQzsaE")
			Url.RawQuery = params.Encode()
			log.Printf("INFO url is %q", Url.String())
			c.String(http.StatusOK, "%s", Url.String())
		} else {
			newUrl := utils.GetParamsSortedUrl(source)

			if refresh == "1" || strings.EqualFold(refresh, "true") {
				db.Delete([]byte(newUrl), nil)
			}
			//c.String(http.StatusOK, "%s", newUrl)
			// check cache
			ret, err := db.Get([]byte(newUrl), nil)
			if nil != err {
				log.Printf("ERR get key failed: %v, %v", newUrl, err)
				go utils.CacheRes(newUrl, db, nil)
				c.Redirect(http.StatusTemporaryRedirect, source)
			} else {
				if len(ret) == 0 {
					// no cache found
					// redirect first and then fetch the resource
					go utils.CacheRes(newUrl, db, nil)
					c.Redirect(http.StatusTemporaryRedirect, source)
				} else {
					wr := utils.DBEntry{}
					if err := utils.GetInterface(ret, &wr); nil != err {
						log.Printf("ERR failed process cached content %v, %v", err, source)
						c.Redirect(http.StatusTemporaryRedirect, source)
					} else {
						for k, v := range wr["header"].(http.Header) {
							c.Writer.Header().Set(k, strings.Join(v, ""))
						}
						c.Writer.Header().Del("Date")
						c.Writer.Write(wr["body"].([]byte))
						log.Printf("INFO success response with cache %#v", newUrl)
					}
				}
			}
		}
	})
	router.Run(":" + strconv.Itoa(env.WebPort))
}
