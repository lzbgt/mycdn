package main

import (
	"mycdn/toolbox"
	"net/http"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

func startApiSrv() {
	// api server
	router := gin.Default()

	router.GET("/addmn", func(c *gin.Context) {
		name := c.Query("name")
		ip := c.Query("ip")
		if len(ip) == 0 {
			c.String(http.StatusOK, "ERR no ip provided")
			return
		}
		ips := strings.Split(ip, ",")

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

	router.GET("/offline", func(c *gin.Context) {
		host := c.Query("host")
		if host != "" {
			Offline(host)
		}
		c.String(http.StatusOK, "done")
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
		ret := map[string]interface{}{
			"ip2mn":      mapIPToMN,
			"uid2mn":     mapUidToMNMap,
			"uid2origin": mapVNameToSourceName,
		}

		c.JSON(http.StatusOK, ret)
	})

	router.GET("/del", func(c *gin.Context) {
		obj := c.Query("obj")
		key1 := c.Query("k1")
		key2 := c.Query("k2")

		if obj == "vname" && key1 != "" {
			delete(mapVNameToSourceName, key1)
		} else if obj == "node" && key1 != "" {
			if key2 == "" && len(mapUidToMNMap[key1]) == 0 {
				delete(mapUidToMNMap, key1)
			} else if key2 != "" {
				delete(mapUidToMNMap[key1], key2)
			}
		} else {
			c.String(http.StatusOK, "usage: /del?obj=<vname|node>&k1=<>&k2=<>")
			return
		}

		c.String(http.StatusOK, "done")
		saveBindingInfo()
		saveNodeInfo()

	})

	router.Run(":" + strconv.Itoa(env.ApiPort))
}

// new Massive Node
func NewMassiveNode(uid, ip string) (mn *MassiveNode) {
	mn = &MassiveNode{ip}
	mapIPToMN[ip] = mn
	if uid != "" {
		if _, ok := mapUidToMNMap[uid]; !ok {
			mapUidToMNMap[uid] = make(map[string]*MassiveNode)
		}

		if _, ok := mapUidToMNMap[uid][ip]; !ok {

			if _, ok := mapLBRing[uid]; !ok {
				mapLBRing[uid] = &utils.LBRing{}
			}
			mapLBRing[uid].Add(mn)
			if v, ok := mapLBRingFailed[uid]; ok && v != nil {
				v.Remove(mn)
			}
			log.Info("massive node comes online ", mn.Ip)
		}

		mapUidToMNMap[uid][ip] = mn
	}

	return mn
}

//
func Offline(hostIp string) {
BRK:
	for k, v := range mapLBRing {
		if v == nil || v.List == nil {
			continue
		}
		for h := v.List.Front(); h != nil; h = h.Next() {
			mn := h.Value.(*MassiveNode)
			if mn.Ip == hostIp {
				v.Remove(mn)
				if _, ok := mapLBRingFailed[k]; !ok {
					mapLBRingFailed[k] = &utils.LBRing{}
				}
				mapLBRingFailed[k].Add(mn)
				log.Info("massive node goes offline ", mn.Ip)
				break BRK
			}
		}
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
