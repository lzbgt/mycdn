package utils

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
)

func GetParamsSortedUrl(source string) string {
	Url, _ := url.Parse(source)
	sourceParams, _ := url.ParseQuery(Url.RawQuery)
	if len(sourceParams) == 0 {
		return source
	}

	keys := make([]string, len(sourceParams))
	i := 0
	for k, _ := range sourceParams {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	newUrl := Url.Scheme + "://" + Url.Host + Url.Path + "?"
	params := make([]string, len(keys))
	i = 0
	for _, v := range keys {
		params[i] = v + "=" + strings.Join(sourceParams[v], "")
		i++
	}
	newUrl += strings.Join(params, "&")

	return newUrl
}

func CacheRes(url string, db *leveldb.DB, dataCh chan DBEntry) error {
	// fetch first and save
	if resp, err := http.Get(url); nil != err {
		return err
	} else {
		defer resp.Body.Close()
		wr := DBEntry{}
		wr["header"] = resp.Header
		if contents, err := ioutil.ReadAll(resp.Body); nil != err {
			log.Printf("ERR failed to read http body %v, %v", err, url)
			return err
		} else {
			wr["body"] = contents
		}
		if nil != dataCh {
			dataCh <- wr
		}
		/*// instead of write a JOSN object, I write the two items separately for levarage the io.Copy
		if n, err := io.Copy((DBWriter{db, []byte(url)}), resp.Body); nil != err {
			log.Printf("ERR failed to cache %v, %v", err, url)
			return err
		} */
		b := GetBytes(&wr)
		if err := db.Put([]byte(url), b, nil); nil != err {
			log.Printf("ERR failed to cache, db.Put: %v, %v", err, url)
			return err
		} else {
			log.Printf("INFO cache success size %v, url %v,", len(b), url)
			return nil
		}
	}
}
