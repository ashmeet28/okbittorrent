package main

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"log"
	"os"
	"sync"
)

var torrentInfo struct {
	infoDictHash []byte
	name         string
	pieceLength  int
	pieceHashes  [][]byte
	isSingleFile bool
	fileLength   int
	files        []struct {
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

func torrentInfoFillFromBenInfo(benInfo map[string]any) {
	torrentInfoMu.Lock()

	benBuf, err := BencodeEncode(benInfo)
	if err != nil {
		log.Fatalln(errors.New("unable to encode info"))
	}

	benBufSHA1Sum := sha1.Sum(benBuf)

	torrentInfo.infoDictHash = benBufSHA1Sum[:]

	benName, ok := benInfo["name"].([]byte)
	if !ok {
		log.Fatalln(errors.New("unable to get name"))
	}

	// TODO: Validate name string

	torrentInfo.name = string(benName)

	benPieceLength, ok := benInfo["piece length"].(int64)
	if !ok {
		log.Fatalln(errors.New("unable to get piece length"))
	}
	if benPieceLength < 1 || benPieceLength > 1_000_000_000 {
		log.Fatalln(errors.New("invalid piece length"))
	}
	torrentInfo.pieceLength = int(benPieceLength)

	benPieces, ok := benInfo["pieces"].([]byte)
	if !ok {
		log.Fatalln(errors.New("unable to get pieces"))
	}

	if len(benPieces) == 0 || (len(benPieces)%20) != 0 {
		log.Fatalln(errors.New("invalid pieces"))
	}

	for i := 0; i < len(benPieces); i += 20 {
		torrentInfo.pieceHashes = append(torrentInfo.pieceHashes,
			benPieces[i:i+20])
	}

	benLength, ok := benInfo["length"].(int64)
	if !ok {
		log.Fatalln(errors.New("unable to get length"))
	}
	if benLength < 1 || benLength > 1_000_000_000_000 {
		log.Fatalln(errors.New("invalid length"))
	}
	torrentInfo.fileLength = int(benLength)

	torrentInfo.isSingleFile = true

	// TODO: Handle multiple files torrent

	torrentInfoMu.Unlock()
}

func torrentTrackersFillFromFile(extraTrackersFilePath string) {
	extraTrackersFileData, err := os.ReadFile(extraTrackersFilePath)
	if err != nil {
		log.Fatalln(errors.New("unable to read extra trackers file"))
	}

	torrentTrackersMu.Lock()

	for t := range bytes.SplitSeq(extraTrackersFileData, []byte{0x0a}) {
		if len(t) != 0 {
			torrentTrackers = append(torrentTrackers, string(t))
		}
	}

	torrentTrackersMu.Unlock()
}

func main() {
	magneticLink := os.Args[1]
	torrentFilePath := os.Args[2]
	// downloadDirectoryPath := os.Args[3]
	extraTrackersFilePath := os.Args[4]
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
				errors.New("unable to get info"))
		}
		torrentInfoFillFromBenInfo(benInfo)
	} else {
		log.Fatalln(errors.New("invalid arguments"))
	}

	if extraTrackersFilePath != "-" {
		torrentTrackersFillFromFile(extraTrackersFilePath)
	}
}
