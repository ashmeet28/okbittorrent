package main

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func main() {
	torrentFileData, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	rootDictInterface, _, err := BencodeDecode(torrentFileData)
	if err != nil {
		log.Fatalln(err)
	}

	rootDict, ok := rootDictInterface.(map[string]any)
	if !ok {
		log.Fatalln(errors.New("invalid data inside torrent file"))
	}

	infoDict, ok := rootDict["info"].(map[string]any)
	if !ok {
		log.Fatal(errors.New("info dict not found"))
	}

	infoDictData, err := BencodeEncode(infoDict)

	if err != nil {
		log.Fatalln(errors.New("internal error"))
	}

	s := sha1.Sum((infoDictData))
	fmt.Println(sliceToEscapedQuery(s[:]))
	fmt.Println(getPeersFromTracker(s[:]))
}

func sliceToEscapedQuery(d []byte) string {
	s := strings.ToUpper(hex.EncodeToString(d))
	var es string
	for i := 0; (i + 2) <= len(s); i += 2 {
		es = es + "%" + s[i:i+2]
	}
	return es
}

func getPeersFromTracker(infoHash []byte) []string {
	resp, err := http.Get(os.Args[2] +
		"/?info_hash=" + sliceToEscapedQuery(infoHash))
	if err != nil {
		log.Fatalln(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatalln("unable to get data from tracker")
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	rootDictInterface, _, err := BencodeDecode(b)
	if err != nil {
		log.Fatalln(err)
	}
	rootDict, ok := rootDictInterface.(map[string]any)
	if !ok {
		log.Fatalln(errors.New("invalid data returned from tracker"))
	}
	peersInterface, ok := rootDict["peers"]
	if !ok {
		log.Fatalln(errors.New("peers not found in tracker response"))
	}
	peersCompact, ok := peersInterface.([]byte)
	if !ok {
		log.Fatalln(errors.New("peers not found in tracker response"))
	}
	var peers []string
	for i := 0; (i + 6) <= len(peersCompact); i += 6 {
		peers = append(peers,
			strconv.FormatInt(int64(peersCompact[i]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+1]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+2]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+3]), 10)+":"+
				strconv.FormatInt((int64(peersCompact[i+4])*256)+
					int64(peersCompact[i+5]), 10))
	}
	return peers
}
