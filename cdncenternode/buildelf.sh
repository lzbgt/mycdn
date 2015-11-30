#!/bin/sh
mv -f cdncenternode_elf cdncenternode_elf.bak; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o cdncenternode_elf; goupx cdncenternode_elf; scp cdncenternode_elf root@121.43.154.122:/root/blu/
