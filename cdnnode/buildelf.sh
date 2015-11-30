#!/bin/sh
mv -f cdnnode_elf cdnnode_elf.bak; CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o cdnnode_elf; goupx cdnnode_elf; scp cdnnode_elf root@121.40.244.135:/root/blu/
