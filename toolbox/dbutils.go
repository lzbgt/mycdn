package utils

import (
	"bytes"
	"encoding/gob"
	"log"
	"net/http"
)

type DBEntry map[string]interface{}

func GobRegister(values ...interface{}) {
	for _, v := range values {
		gob.Register(v)
	}

}
func GetBytes(data interface{}) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return buf.Bytes()
}

func GetInterface(bts []byte, data interface{}) error {
	buf := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(data)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	GobRegister(DBEntry{})
	GobRegister(http.Header{})
}
