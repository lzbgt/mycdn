package main

import (
	"flag"
	"fmt"

	"cdntool/aliyungo/dns"
)

// const ACCESS_KEY_ID = "2K34F3PYn9wgmL44"
// const ACCESS_KEY_SECRET = "KsTQD1l4pci8pylixBVq0k3n9ZodqU"
const ACCESS_KEY_ID = "VMfUUo6ToZwtdLZN"
const ACCESS_KEY_SECRET = "IeWZweulcqONbhlxtczx6J5LZZINnx"

// cmd args
var fDomain, fOperation, fRR, fRType, fRValue string

func init() {
	flag.StringVar(&fDomain, "domain", "leither.cn", "domain name")
	flag.StringVar(&fOperation, "act", "list", "actions: list, add, del")
	flag.StringVar(&fRR, "RR", "", "host record: @, *, <sub-name>")
	flag.StringVar(&fRType, "type", "A", "A, NS, MX, TXT, CNAME, SRV, AAAA, REDIRECT_URL, FORWORD_URL")
	flag.StringVar(&fRValue, "value", "", "record value")
	flag.Parse()
}

func main() {
	client := dns.NewClient(ACCESS_KEY_ID, ACCESS_KEY_SECRET)

	//
	switch fOperation {
	case "add":
		param := &dns.AddDomainRecordArgs{}
		param.RR = fRR
		param.Type = fRType
		param.Value = fRValue
		param.DomainName = fDomain
		resp, _ := client.AddDomainRecord(param)
		fmt.Println(resp)
	case "del":
		param := &dns.DeleteSubDomainRecordsArgs{}
		param.DomainName = fDomain
		param.RR = fRR
		param.Type = fRType
		resp, _ := client.DeleteSubDomainRecords(param)
		fmt.Println(resp)
	}
	// list domains
	param := &dns.DescribeDomainRecordsArgs{DomainName: fDomain}
	param.PageSize = 500
	recs, _ := client.DescribeDomainRecords(param)
	// fmt.Printf("%v", recs)
	for _, v := range recs.DomainRecords.Record {
		if /*v.RR == "cdn" &&*/ !v.Locked /*&& v.Type == "A"*/ {
			fmt.Printf("%s,%s,%s\n", v.Type, v.RR, v.Value)
		}
	}
}
