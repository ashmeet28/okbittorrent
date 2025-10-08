package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

var torrentPeerId []byte

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
	address string
}

var torrentPeers []torrentPeer
var torrentPeersMu sync.Mutex

var torrentTrackers []string
var torrentTrackersMu sync.Mutex

var peersFinderActiveTrackers []string
var peersFinderActiveTrackersMu sync.Mutex

var peersFinderMaxActiveTrackers int = 10

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

func peersFinderConnectTracker(trackerURL string) {
	peersFinderActiveTrackersMu.Lock()
	if slices.Contains(peersFinderActiveTrackers, trackerURL) {
		peersFinderActiveTrackersMu.Unlock()
		return
	} else {
		peersFinderActiveTrackers = append(peersFinderActiveTrackers,
			trackerURL)
		peersFinderActiveTrackersMu.Unlock()
	}

	deleteTrackerURLFromActiveTrackers := func() {
		peersFinderActiveTrackersMu.Lock()

		i := slices.Index(peersFinderActiveTrackers, trackerURL)
		if i == -1 {
			log.Fatalln(errors.New("expected tracker url in the active trackers slice"))
		}
		peersFinderActiveTrackers = append(
			peersFinderActiveTrackers[:i], peersFinderActiveTrackers[i+1:]...)

		peersFinderActiveTrackersMu.Unlock()
	}

	sliceToEscapedQuery := func(d []byte) string {
		s := strings.ToUpper(hex.EncodeToString(d))
		var es string
		for i := 0; (i + 2) <= len(s); i += 2 {
			es = es + "%" + s[i:i+2]
		}
		return es
	}

	torrentInfoMu.Lock()

	res, err := http.Get(trackerURL +
		"/?info_hash=" + sliceToEscapedQuery(torrentInfo.infoDictHash) +
		"&peer_id=" + sliceToEscapedQuery(torrentPeerId))

	torrentInfoMu.Unlock()

	if err != nil {
		deleteTrackerURLFromActiveTrackers()
		return
	}

	body, err := io.ReadAll(res.Body)

	if err := res.Body.Close(); err != nil {
		deleteTrackerURLFromActiveTrackers()
		return
	}
	if (res.StatusCode != http.StatusOK) || (err != nil) {
		deleteTrackerURLFromActiveTrackers()
		return
	}

	ben, extraData, err := BencodeDecode(body)
	if len(extraData) != 0 || err != nil {
		deleteTrackerURLFromActiveTrackers()
		return
	}

	benRoot, ok := ben.(map[string]any)
	if !ok {
		deleteTrackerURLFromActiveTrackers()
		return
	}

	benPeers, ok := benRoot["peers"].([]byte)
	if !ok {
		deleteTrackerURLFromActiveTrackers()
		return
	}

	for i := 0; (i + 6) <= len(benPeers); i += 6 {
		peerIPAddr := strconv.FormatInt(int64(benPeers[i]), 10) + "." +
			strconv.FormatInt(int64(benPeers[i+1]), 10) + "." +
			strconv.FormatInt(int64(benPeers[i+2]), 10) + "." +
			strconv.FormatInt(int64(benPeers[i+3]), 10) + ":" +
			strconv.FormatInt(
				(int64(benPeers[i+4])*256)+int64(benPeers[i+5]), 10)

		torrentPeersMu.Lock()
		torrentPeersAlreadyHasAddress := false
		for _, p := range torrentPeers {
			if p.address == peerIPAddr {
				torrentPeersAlreadyHasAddress = true
				break
			}
		}
		if !torrentPeersAlreadyHasAddress {
			torrentPeers = append(
				torrentPeers, torrentPeer{address: peerIPAddr})
		}
		torrentPeersMu.Unlock()
	}

	deleteTrackerURLFromActiveTrackers()
}

func peersFinderStart() {
	nextTrackerIndex := 0

	for {
		torrentTrackersMu.Lock()
		peersFinderActiveTrackersMu.Lock()

		if nextTrackerIndex >= len(torrentTrackers) {
			nextTrackerIndex = 0
		}

		if len(torrentTrackers) != 0 &&
			len(peersFinderActiveTrackers) < peersFinderMaxActiveTrackers {
			go peersFinderConnectTracker(torrentTrackers[nextTrackerIndex])
			nextTrackerIndex++
		}

		torrentTrackersMu.Unlock()
		peersFinderActiveTrackersMu.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func main() {
	magneticLink := os.Args[1]
	torrentFilePath := os.Args[2]
	// downloadDirectoryPath := os.Args[3]
	extraTrackersFilePath := os.Args[4]
	// extraPeersFilePath := os.Args[5]

	torrentPeerId = make([]byte, 20)
	if _, err := rand.Read(torrentPeerId); err != nil {
		log.Fatalln(errors.New("unable to get random bytes"))
	}

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

	peersFinderStart()
}
