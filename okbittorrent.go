package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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
	pieces [][]byte
}
var torrentInfoMu sync.Mutex

type torrentPeer struct {
	address  string
	isActive bool
}

var torrentPeers []torrentPeer
var torrentPeersMu sync.Mutex

type torrentTracker struct {
	address  string
	isActive bool
}

var torrentTrackers []torrentTracker
var torrentTrackersMu sync.Mutex

var peersFinderMaxActiveTrackers int = 10

var torrentDownloaderMaxActivePeers int = 300

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
	torrentInfo.pieces = make([][]byte, len(torrentInfo.pieceHashes))

	benLength, ok := benInfo["length"].(int64)
	if !ok {
		log.Fatalln(errors.New("unable to get length"))
	}
	if benLength < 1 || benLength > 1_000_000_000_000 {
		log.Fatalln(errors.New("invalid length"))
	}
	if ((benLength < benPieceLength) && len(torrentInfo.pieceHashes) != 1) ||
		((benLength%benPieceLength) == 0 &&
			int(benLength/benPieceLength) != len(torrentInfo.pieceHashes)) ||
		((benLength%benPieceLength) != 0 &&
			(int(benLength/benPieceLength)+1) != len(torrentInfo.pieceHashes)) {
		log.Fatalln(errors.New("invalid length"))
	}
	torrentInfo.fileLength = int(benLength)

	torrentInfo.isSingleFile = true

	// TODO: Handle multiple files torrent

	torrentInfoMu.Unlock()
}

func torrentInfoGetPiecesBitfield() []byte {
	torrentInfoMu.Lock()
	bf := make([]byte, len(torrentInfo.pieces)/8)
	if (len(torrentInfo.pieces) < 8) || ((len(torrentInfo.pieces) % 8) != 0) {
		bf = append(bf, 0)
	}
	for i, p := range torrentInfo.pieces {
		if len(p) != 0 {
			if i == 0 {
				bf[0] = bf[0] | (0b1000_0000 >> (i % 8))
			} else {
				bf[i/8] = bf[i/8] | (0b1000_0000 >> (i % 8))
			}

		}
	}
	torrentInfoMu.Unlock()
	return bf
}

func torrentTrackersFillFromFile(extraTrackersFilePath string) {
	extraTrackersFileData, err := os.ReadFile(extraTrackersFilePath)
	if err != nil {
		log.Fatalln(errors.New("unable to read extra trackers file"))
	}

	torrentTrackersMu.Lock()

	for trackerURL := range bytes.SplitSeq(extraTrackersFileData, []byte{0x0a}) {
		if len(trackerURL) != 0 {
			doesExists := false
			for _, t := range torrentTrackers {
				if t.address == string(trackerURL) {
					doesExists = true
					break
				}
			}
			if !doesExists {
				torrentTrackers = append(torrentTrackers,
					torrentTracker{
						address: string(trackerURL), isActive: false})
			}
		}
	}

	torrentTrackersMu.Unlock()
}

func peersFinderConnectTracker(trackerURL string) {
	deactivateTracker := func() {
		torrentTrackersMu.Lock()
		isActive := true
		for i, t := range torrentTrackers {
			if t.address == trackerURL {
				torrentTrackers[i].isActive = false
				isActive = false
				break
			}
		}
		if isActive {
			log.Fatalln(errors.New("unable to deactivate tracker"))
		}
		torrentTrackersMu.Unlock()
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
		deactivateTracker()
		return
	}

	body, err := io.ReadAll(res.Body)

	if err := res.Body.Close(); err != nil {
		deactivateTracker()
		return
	}
	if (res.StatusCode != http.StatusOK) || (err != nil) {
		deactivateTracker()
		return
	}

	ben, extraData, err := BencodeDecode(body)
	if len(extraData) != 0 || err != nil {
		deactivateTracker()
		return
	}

	benRoot, ok := ben.(map[string]any)
	if !ok {
		deactivateTracker()
		return
	}

	benPeers, ok := benRoot["peers"].([]byte)
	if !ok {
		deactivateTracker()
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
				torrentPeers,
				torrentPeer{
					address: peerIPAddr, isActive: false})
		}
		torrentPeersMu.Unlock()
	}

	deactivateTracker()
}

func peersFinderStart() {
	nextTrackerIndex := 0

	for {
		torrentTrackersMu.Lock()

		if nextTrackerIndex >= len(torrentTrackers) {
			nextTrackerIndex = 0
		}

		currentlyActiveTrackers := 0
		for _, t := range torrentTrackers {
			if t.isActive {
				currentlyActiveTrackers++
			}
		}

		if (len(torrentTrackers) != 0) &&
			(currentlyActiveTrackers < peersFinderMaxActiveTrackers) {

			if !torrentTrackers[nextTrackerIndex].isActive {
				torrentTrackers[nextTrackerIndex].isActive = true
				go peersFinderConnectTracker(
					torrentTrackers[nextTrackerIndex].address)
			}
			nextTrackerIndex++

		}

		torrentTrackersMu.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func torrentDownloaderConnectPeer(peerIPAddr string) {
	// fmt.Println("A: " + peerIPAddr)
	deactivatePeer := func() {
		torrentPeersMu.Lock()
		isActive := true
		for i, t := range torrentPeers {
			if t.address == peerIPAddr {
				torrentPeers[i].isActive = false
				isActive = false
				break
			}
		}
		if isActive {
			log.Fatalln(errors.New("unable to deactivate peer"))
		}
		torrentPeersMu.Unlock()
		// fmt.Println("D: " + peerIPAddr)
	}

	conn, err := net.DialTimeout("tcp", peerIPAddr, time.Second*15)
	if err != nil {
		deactivatePeer()
		return
	}
	// fmt.Println("C: " + peerIPAddr)

	w := func(b []byte) bool {
		conn.SetDeadline(time.Now().Add(time.Minute * 15))
		n, err := conn.Write(b)
		if err != nil || n != len(b) {
			return false
		}
		return true
	}

	r := func() (byte, bool) {
		b := []byte{0}
		conn.SetDeadline(time.Now().Add(time.Minute * 5))
		n, _ := conn.Read(b)
		if n == 1 {
			return b[0], true
		} else {
			return b[0], false

		}
	}

	wMsgUnchock := func() bool {
		return w([]byte{0, 0, 0, 1, 1})
	}

	wMsgInterested := func() bool {
		return w([]byte{0, 0, 0, 1, 2})
	}

	wMsgRequest := func() bool {
		var wBuf []byte
		wBuf = binary.BigEndian.AppendUint32(wBuf, 13)
		wBuf = append(wBuf, 6)
		wBuf = binary.BigEndian.AppendUint32(wBuf, 10)
		wBuf = binary.BigEndian.AppendUint32(wBuf, 0)
		torrentInfoMu.Lock()
		wBuf = binary.BigEndian.AppendUint32(wBuf,
			uint32(torrentInfo.pieceLength))
		torrentInfoMu.Unlock()
		return w(wBuf)
	}

	wMsgBitfield := func() bool {
		var wBuf []byte
		wBuf = binary.BigEndian.AppendUint32(wBuf,
			uint32(len(torrentInfoGetPiecesBitfield())+1))
		wBuf = append(wBuf, 5)
		wBuf = append(wBuf, torrentInfoGetPiecesBitfield()...)
		return w(wBuf)
	}

	var wBuf []byte
	wBuf = []byte{19}
	wBuf = append(wBuf, []byte("BitTorrent protocol")...)
	wBuf = append(wBuf, make([]byte, 8)...)
	torrentInfoMu.Lock()
	wBuf = append(wBuf, torrentInfo.infoDictHash...)
	torrentInfoMu.Unlock()
	wBuf = append(wBuf, torrentPeerId...)

	if ok := w(wBuf); !ok {
		deactivatePeer()
		return
	}

	wMsgBitfield()
	wMsgUnchock()
	wMsgInterested()
	wMsgRequest()

	var rBuf []byte
	for len(rBuf) < len(wBuf) {
		b, ok := r()
		if !ok {
			deactivatePeer()
			return
		}
		rBuf = append(rBuf, b)
	}

	if !(slices.Equal(wBuf[:20], rBuf[:20]) &&
		slices.Equal(wBuf[28:36], rBuf[28:36])) {
		deactivatePeer()
		return
	}
	fmt.Println("H: " + peerIPAddr)

	for b, ok := r(); ok; {
		fmt.Println(b)
		time.Sleep(1 * time.Second)
	}
	deactivatePeer()
}

func torrentDownloaderStart() {
	nextPeerIndex := 0

	for {
		torrentPeersMu.Lock()

		if nextPeerIndex >= len(torrentPeers) {
			nextPeerIndex = 0
		}

		currentlyActivePeers := 0
		for _, p := range torrentPeers {
			if p.isActive {
				currentlyActivePeers++
			}
		}

		if (len(torrentPeers) != 0) &&
			(currentlyActivePeers < torrentDownloaderMaxActivePeers) {

			if !torrentPeers[nextPeerIndex].isActive {
				torrentPeers[nextPeerIndex].isActive = true
				go torrentDownloaderConnectPeer(
					torrentPeers[nextPeerIndex].address)
			}
			nextPeerIndex++

		}

		torrentPeersMu.Unlock()

		time.Sleep(100 * time.Millisecond)
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

	go peersFinderStart()
	torrentDownloaderStart()
}
