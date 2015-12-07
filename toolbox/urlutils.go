package utils

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	log "github.com/Sirupsen/logrus"

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

func SaveShortUrl(id *uint64, source string, db *leveldb.DB) string {
	newId := atomic.AddUint64(id, 1)
	key1 := SURL_PREFIX + strconv.FormatUint(newId, 10)
	key2 := SURL_PREFIX + ShortURLEncode(newId)

	if err := db.Put([]byte(key1), []byte(source), nil); nil != err {
		log.Errorf("Failed to save shorturl: %v, %v, %v", key1, key2, err)
	}

	return key2
}

// TODO
func ParseShortUrl(path string) (string, bool) {
	if len(path) < (len(SURL_PREFIX) + 1) {
		return "", false
	}
	if strings.EqualFold(path[:len(SURL_PREFIX)], SURL_PREFIX) {
		//db.Get([]byte())
	}

	return "", false
}

// url shorter
const SURL_PREFIX = "axa"
const SURL_ALPHABET = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const SURL_BASE = uint64(len(SURL_ALPHABET))

func ShortURLEncode(num uint64) string {
	b := bytes.Buffer{}
	for num != 0 {
		idx := num % SURL_BASE
		b.WriteString(SURL_ALPHABET[idx : idx+1])

		num /= SURL_BASE
	}

	for i, j := 0, len(b.Bytes())-1; i < j; i, j = i+1, j-1 {
		b.Bytes()[i], b.Bytes()[j] = b.Bytes()[j], b.Bytes()[i]
	}

	return string(b.Bytes())
}

func ShortURLDecode(str string) uint64 {
	num := uint64(0)

	for i := 0; i < len(str); i++ {
		num = num*SURL_BASE + uint64(strings.Index(SURL_ALPHABET, str[i:i+1]))
	}

	return num
}
