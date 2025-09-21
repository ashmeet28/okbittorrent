package main

import (
	"errors"
	"log"
	"os"
)

type torrentInfo struct {
}

type torrentPeers []torrentPeer

type torrentPeer struct {
}

var torrentTrackers []string

var torrentNewPeers chan torrentPeer

var torrentNewTrackers chan string

func infoDictIsValid(map[string]any) bool {
	return false
}

func main() {
	magneticLink := os.Args[1]
	torrentFilePath := os.Args[2]
	downloadDirectoryPath := os.Args[3]
	extraTrackersFilePath := os.Args[4]
	extraPeersFilePath := os.Args[5]

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

		benRootDict, ok := torrentBencode.(map[string]any)
		if !ok {
			log.Fatalln(errors.New("invalid bencode format"))
		}

		benInfoDict, ok := benRootDict["info"].(map[string]any)
		if !ok {
			log.Fatalln(
				errors.New("unable to find info dictionary in torrent file"))
		}
	} else {
		log.Fatalln(errors.New("invalid arguments"))
	}
}
