package main

import (
	"flag"
	"mycdn/toolbox"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
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
	// center node addr: ip or name
	CtrlAddr string
	LogLevel string
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
	log.SetOutput(os.Stderr)
	var logLevels []string = make([]string, 0, 10)
	logLevels = append(logLevels, log.DebugLevel.String())
	logLevels = append(logLevels, log.InfoLevel.String())
	logLevels = append(logLevels, log.WarnLevel.String())
	logLevels = append(logLevels, log.ErrorLevel.String())
	logLevels = append(logLevels, log.FatalLevel.String())
	logLevels = append(logLevels, log.PanicLevel.String())

	flag.IntVar(&env.RPCPort, "rpc", 2048, "rpc port")
	flag.IntVar(&env.WebPort, "web", 80, "web port")
	flag.StringVar(&env.HostIP, "host", "127.0.0.1", "host IP")
	flag.StringVar(&env.DBPath, "db", "", "db path")
	flag.StringVar(&env.LogLevel, "log", log.DebugLevel.String(), "log level: "+strings.Join(logLevels, ","))
	flag.StringVar(&env.CtrlAddr, "center", "ctrl.cdn.leither.cn:81", "center node addr")

	flag.Parse()
	lvl, err := log.ParseLevel(env.LogLevel)
	if nil == err {
		log.SetLevel(lvl)
	} else {
		log.SetLevel(log.DebugLevel)
		log.Warn("invalid log level, use default")
	}

	if env.DBPath == "" {
		env.DBPath = filepath.Dir(os.Args[0]) + "/DB"
	}
	os.MkdirAll(env.DBPath, 0777)
	db, err = leveldb.OpenFile(env.DBPath, nil)
	if err != nil {
		panic(err)
	}

	// register to ctrl.cdn.leither.cn every 1 minute untill success
	if AddToCtrlNode(env.CtrlAddr) == false {
		ticker := time.NewTicker(time.Second * 60 * 1)
		go func() {
			for _ = range ticker.C {
				if AddToCtrlNode(env.CtrlAddr) {
					return
				}
			}
		}()
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
		Offline()
		syscall.Exit(0)
	}()

	// add itself to ctrl node

	service := hprose.NewHttpService()
	service.AddMethods(DService{})
	go http.ListenAndServe(":"+strconv.Itoa(env.RPCPort), service)

	// web server
	router := gin.Default()
	router.HEAD("/alive", func(c *gin.Context) {
		c.String(http.StatusOK, "Yes")
	})

	router.GET("/alive", func(c *gin.Context) {
		c.String(http.StatusOK, "Yes")
	})

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
			log.Infof("url is %q", Url.String())
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
				log.Errorf("get key failed: %v, %v", newUrl, err)
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
						log.Errorf("failed process cached content %v, %v", err, source)
						c.Redirect(http.StatusTemporaryRedirect, source)
					} else {
						header := wr["header"].(http.Header)
						for k, v := range header {
							c.Writer.Header().Set(k, strings.Join(v, ""))
						}

						c.Writer.Header().Del("Date")
						c.Writer.Write(wr["body"].([]byte))
						log.Infof("success response with cache, len: %v, url: %v", header["Content-Length"], newUrl)
					}
				}
			}
		}
	})
	router.Run(":" + strconv.Itoa(env.WebPort))
}

func AddToCtrlNode(ctrlAddr string) bool {
	if _, err := http.Get("http://" + env.CtrlAddr + "/addmn?ips=" + env.HostIP); nil != err {
		log.Error("Failed connect to center node: ", env.CtrlAddr, err)
		return false
	} else {
		log.Info("Success registered to center node ", env.CtrlAddr)
		return true
	}
}

func Offline() {
	if _, err := http.Get("http://" + env.CtrlAddr + "/offline?host=" + env.HostIP); nil != err {
		log.Error("Failed connect to center node: ", env.CtrlAddr, err)
	} else {
		log.Info("Success offline from center node ", env.CtrlAddr)
	}
}
