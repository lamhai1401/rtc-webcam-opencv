package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc"
	"gocv.io/x/gocv"
	cv "gocv.io/x/gocv"
)

var imgChan = make(chan *gocv.Mat)

func client1() {
	peerConnection, err := webrtc.NewPeerConnection(config)

	if err != nil {
		panic(err)
	}

	// create Trach the we send video back to
	outputTrack, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion")
	if err != nil {
		panic(err)
	}

	// Add this newly created track to the PeerConnection
	if _, err = peerConnection.AddTrack(outputTrack); err != nil {
		panic(err)
	}

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("ICE Connection State has changed: %s\n", connectionState.String())
	})

	// get webcam here
	webcam, _ := cv.OpenVideoCapture(0)
	img := gocv.NewMat()

	// img.

	defer webcam.Close()
	defer img.Close()

	go func() {
		webcam.Read(&img)
		imgChan <- &img
	}()

	peerConnection.OnTrack(func(track *webrtc.Track, receiver *webrtc.RTPReceiver) {
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		for {
			// Read RTP packets  sent to Pion
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}
			fmt.Print(rtp)
			// Replace the SSRC with the SSRC of the outbound track.
			// The only change we are making replacing the SSRC, the RTP packets are unchanged otherwise

			rtp.SSRC = outputTrack.SSRC()
			rtp.PayloadType = webrtc.DefaultPayloadTypeVP8

			if writeErr := outputTrack.WriteRTP(rtp); writeErr != nil {
				panic(writeErr)
			}
		}
	})

	// Create offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// send to signal
	jsonData := map[string]string{
		"sdp":  offer.SDP,
		"type": offer.Type.String(),
	}
	jsonValue, _ := json.Marshal(jsonData)
	response, err := http.Post(
		"https://localhost:8080/client1",
		"application/json",
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		fmt.Printf("The HTTP request failed with error %s\n", err)
	} else {
		data, _ := ioutil.ReadAll(response.Body)
		fmt.Println(string(data))
	}
	// orther way
	// jsonData := map[string]string{"firstname": "Nic", "lastname": "Raboy"}
	// jsonValue, _ := json.Marshal(jsonData)
	// request, _ := http.NewRequest("POST", "https://httpbin.org/post", bytes.NewBuffer(jsonValue))
	// request.Header.Set("Content-Type", "application/json")
	// client := &http.Client{}
	// response, err := client.Do(request)
	// if err != nil {
	// 	fmt.Printf("The HTTP request failed with error %s\n", err)
	// } else {
	// 	data, _ := ioutil.ReadAll(response.Body)
	// 	fmt.Println(string(data))
	// }
}

func show() {
	webcam, _ := cv.OpenVideoCapture(0)
	window := gocv.NewWindow("Hello")
	img := gocv.NewMat()
	defer webcam.Close()
	defer img.Close()

	for {
		webcam.Read(&img)
		fmt.Print(img.DataPtrFloat64())
		window.IMShow(img)
		window.WaitKey(1)
	}
}
