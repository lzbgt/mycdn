package main

import (
	"mycdn/toolbox"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
)

const REFRESH_PARAMNAME = "_sysrefresh"

func startWebSrv() {
	http.HandleFunc("/", httphandler)
	log.Println("INFO start web server on port ", env.WebPort)
	go func() {
		if err := http.ListenAndServe(":"+strconv.Itoa(env.WebPort), nil); nil != err {
			panic(err)
		}
	}()
}

//
func httphandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("clientAddr: %v, Url:%v", r.RemoteAddr, r.Host+r.RequestURI)
	httpReqInfo := getHttpReqInfo(w, r)

	if httpReqInfo.SourceAddr == "" {
		return
	}

	// refresh
	if httpReqInfo.Refresh {
		db.Delete([]byte(httpReqInfo.SortedUrl), nil)
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
	if mn := GetFastNode(httpReqInfo.HostAddr[0], httpReqInfo.ClientAddr[0]); mn == nil {
		log.Println("WARN no massive node available for ", httpReqInfo.SourceUrl, ", use source only")
		// redirect to source
		http.Redirect(w, r, httpReqInfo.SourceUrl, http.StatusTemporaryRedirect)
	} else {
		Url, err := url.Parse("http://" + mn.Ip + ":" + strconv.Itoa(env.MNPort) + "/getres")
		if nil != err {
			log.Errorf("%v", err)
			return
		}

		//
		params := url.Values{}
		params.Add("bid", httpReqInfo.HostAddr[0])
		params.Add("source", httpReqInfo.SortedUrl)
		if httpReqInfo.Refresh {
			params.Add("refresh", "1")
		}
		Url.RawQuery = params.Encode()
		http.Redirect(w, r, Url.String(), http.StatusTemporaryRedirect)
		log.Infof("redirect to massive node: %v, %v", Url.String(), httpReqInfo.SortedUrl)
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
		log.Errorf("no such dynamic name registered: %v", dname)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Invalid user of LOSCDN. Please register on http://leither.cn"))
	} else {
		// additional port exists
		if len(dname) == 2 {
			sourceAddr += ":" + dname[1]
		}

		ret.SourceAddr = sourceAddr
		ret.SourceUrl = "http://" + sourceAddr + r.RequestURI
		Url, _ := url.Parse(ret.SourceUrl)
		values := Url.Query()
		if t := values.Get(REFRESH_PARAMNAME); strings.EqualFold(t, "1") || strings.EqualFold(t, "true") {
			ret.Refresh = true
			values.Del(REFRESH_PARAMNAME)
			newUrl := Url.Scheme + "://" + Url.Host + Url.Path
			if len(values) == 0 {
				ret.SourceUrl = newUrl
			} else {
				newUrl += "?"
				vs := make([]string, len(values))
				i := 0
				for k, v := range values {
					vs[i] = k + "=" + strings.Join(v, "")
				}
				newUrl += strings.Join(vs, "&")
				ret.SourceUrl = newUrl
			}
		}

		ret.SortedUrl = utils.GetParamsSortedUrl(ret.SourceUrl)
		ret.ClientAddr = strings.Split(r.RemoteAddr, ":")
	}

	return ret
}

func BuildLBRingFromUidMap() map[string]*utils.LBRing {
	ret := make(map[string]*utils.LBRing)
	for k, v := range mapUidToMNMap {
		ret[k] = &utils.LBRing{}
		for _, n := range v {
			ret[k].Add(n)
			log.Info("LBRing: %v->%v", k, n)
		}
	}

	return ret
}

//
func getAliveUrl(ip string) string {
	return "http://" + ip + ":" + strconv.Itoa(env.MNPort) + "/alive"
}

//
func GetFastNode(uid, clientIP string) *MassiveNode {
	if nx := mapLBRing[uid].Next(); nx != nil {
		return nx.(*MassiveNode)
	}
	return nil
}

func RefreshLBRing() {
	for k, v := range mapLBRingFailed {
		if v == nil || v.List == nil {
			continue
		}
		for h := v.List.Front(); h != nil; h = h.Next() {
			mn := h.Value.(*MassiveNode)
			if _, err := http.Head(getAliveUrl(mn.Ip)); err == nil {
				mapLBRingFailed[k].Remove(mn)
				mapLBRing[k].Add(mn)
				log.Warningf("%v comes alive", mn)
			}
		}
	}

	//log.Infof("mapLBRing: %v", mapLBRing)
	for k, v := range mapLBRing {
		if v == nil || v.List == nil {
			continue
		}
		for h := v.List.Front(); h != nil; h = h.Next() {
			mn := h.Value.(*MassiveNode)
			if _, err := http.Head(getAliveUrl(mn.Ip)); err != nil {
				mapLBRing[k].Remove(mn)
				if z, ok := mapLBRingFailed[k]; !ok || z == nil {
					mapLBRingFailed[k] = &utils.LBRing{}
				}
				mapLBRingFailed[k].Add(mn)
				log.Warningf("%v goes down", mn)
			}
		}
	}
}
