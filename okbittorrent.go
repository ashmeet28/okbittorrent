package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
)

var torrentInfo struct {
	dictSHA1Hash     []byte
	name             string
	pieceLength      int
	piecesSHA1Hashes [][]byte
	isSingleFile     bool
	fileLength       int
	files            []struct {
		fileLength int
		filePath   string
	}
}

var torrentInfoMu sync.Mutex

type torrentPeer struct {
}

var torrentPeers []torrentPeer

var torrentPeersMu sync.Mutex

var torrentTrackers []string

var torrentTrackersMu sync.Mutex

func infoDictIsValid(map[string]any) bool {
	return false
}

func torrentInfoFillFromBenInfo(benInfo map[string]any) {
	torrentInfoMu.Lock()

	benBuf, err := BencodeEncode(benInfo)
	if err != nil {
		log.Fatalln(errors.New("unable to encode info dictionary"))
	}

	benBufSHA1Sum := sha1.Sum(benBuf)

	torrentInfo.dictSHA1Hash = benBufSHA1Sum[:]

	benName, ok := benInfo["name"].([]byte)
	if !ok {
		log.Fatalln(errors.New("unable to get name from info dictionary"))
	}

	// TODO: Validate name string

	torrentInfo.name = string(benName)

	benPieceLength, ok := benInfo["piece length"].(int64)
	if !ok {
		log.Fatalln(errors.New("unable to get piece length from info dictionary"))
	}
	torrentInfo.pieceLength = int(benPieceLength)

	fmt.Println(torrentInfo)

	torrentInfoMu.Unlock()
}

func main() {
	magneticLink := os.Args[1]
	torrentFilePath := os.Args[2]
	// downloadDirectoryPath := os.Args[3]
	// extraTrackersFilePath := os.Args[4]
	// extraPeersFilePath := os.Args[5]

	if magneticLink != "-" && torrentFilePath == "-" {

	} else if magneticLink == "-" && torrentFilePath != "-" {
		torrentFileData, err := os.ReadFile(torrentFilePath)
		if err != nil {
			log.Fatalln(errors.New("unable to read torrent file"))
		}

		torrentBencode, torrentFileExtraData, err :=
			BencodeDecode(torrentFileData)
		if len(torrentFileExtraData) != 0 {
			log.Fatalln(errors.New("extra data in torrent file"))
		}
		if err != nil {
			log.Fatalln(errors.New("unable to decode torrent file"))
		}

		benRoot, ok := torrentBencode.(map[string]any)
		if !ok {
			log.Fatalln(errors.New("invalid bencode format"))
		}

		benInfo, ok := benRoot["info"].(map[string]any)
		if !ok {
			log.Fatalln(
				errors.New("unable to find info dictionary in torrent file"))
		}
		torrentInfoFillFromBenInfo(benInfo)
	} else {
		log.Fatalln(errors.New("invalid arguments"))
	}
}
