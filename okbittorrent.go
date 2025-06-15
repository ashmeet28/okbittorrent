package main

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	infoDict := getInfoDictUsingDotTorrentFileArg()

	infoHash := getSHA1SumOfInfoDict(infoDict)

	peersIPAddrs := getPeersUsingTrackerURLArg(infoHash)

	for _, peersIPAddr := range peersIPAddrs {
		_, err := tryToHandshake(infoHash, peersIPAddr)
		if err == nil {
			break
		}
	}
}

func getInfoDictUsingDotTorrentFileArg() map[string]any {
	torrentFileData, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	rootDictInterface, _, err := BencodeDecode(torrentFileData)
	if err != nil {
		log.Fatal(err)
	}

	rootDict, ok := rootDictInterface.(map[string]any)
	if !ok {
		log.Fatal(errors.New("invalid data inside torrent file"))
	}

	infoDict, ok := rootDict["info"].(map[string]any)
	if !ok {
		log.Fatal(errors.New("info dict not found"))
	}

	return infoDict
}

func getSHA1SumOfInfoDict(infoDict map[string]any) []byte {
	infoDictData, err := BencodeEncode(infoDict)
	if err != nil {
		log.Fatal(errors.New("internal error"))
	}

	s := sha1.Sum((infoDictData))
	return s[:]
}

func sliceToEscapedQuery(d []byte) string {
	s := strings.ToUpper(hex.EncodeToString(d))
	var es string
	for i := 0; (i + 2) <= len(s); i += 2 {
		es = es + "%" + s[i:i+2]
	}
	return es
}

func getPeersUsingTrackerURLArg(infoHash []byte) []string {
	urlWithQuery := os.Args[2] +
		"/?info_hash=" + sliceToEscapedQuery(infoHash)
	fmt.Println("connecting to tracker [" + urlWithQuery + "]")

	resp, err := http.Get(urlWithQuery)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatal("unable to get data from tracker")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	rootDictInterface, _, err := BencodeDecode(respBody)
	if err != nil {
		log.Fatal(err)
	}

	rootDict, ok := rootDictInterface.(map[string]any)
	if !ok {
		log.Fatal(errors.New("invalid data returned from tracker"))
	}

	peersInterface, ok := rootDict["peers"]
	if !ok {
		log.Fatal(errors.New("peers not found in tracker response"))
	}

	peersCompact, ok := peersInterface.([]byte)
	if !ok {
		log.Fatal(errors.New("peers not found in tracker response"))
	}

	var peersIPAddrs []string
	for i := 0; (i + 6) <= len(peersCompact); i += 6 {
		peersIPAddrs = append(peersIPAddrs,
			strconv.FormatInt(int64(peersCompact[i]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+1]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+2]), 10)+"."+
				strconv.FormatInt(int64(peersCompact[i+3]), 10)+":"+
				strconv.FormatInt((int64(peersCompact[i+4])*256)+
					int64(peersCompact[i+5]), 10))
	}
	return peersIPAddrs
}

func tryToHandshake(infoHash []byte, peerIPAddr string) (net.Conn, error) {
	fmt.Println("connecting to " + peerIPAddr)

	conn, err := net.DialTimeout("tcp", peerIPAddr, time.Second*5)
	if err != nil {
		fmt.Println(errors.New("error while connecting to peer"))
		return conn, err
	}

	conn.Write([]byte{19})
	conn.Write([]byte("BitTorrent protocol"))
	conn.Write(make([]byte, 8))
	conn.Write(infoHash)

	peerId := make([]byte, 20)
	rand.Read(peerId)
	conn.Write(peerId)

	conn.Write([]byte{0, 0, 0, 1, 1})
	conn.Write([]byte{0, 0, 0, 1, 2})

	for {
		b := make([]byte, 8)
		n, err := conn.Read(b)
		if err != nil {
			fmt.Println(errors.New("error while handshaking"))
			return conn, err
		}
		fmt.Println(b[:n])
	}
}
