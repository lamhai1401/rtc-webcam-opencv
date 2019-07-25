package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math/rand"
	"time"

	"net/http"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc"
	"github.com/pion/webrtc/pkg/media"
	"github.com/poi5305/go-yuv2webRTC/screenshot"
	vpxEncoder "github.com/poi5305/go-yuv2webRTC/vpx-encoder"
	"gocv.io/x/gocv"
)

type Session struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

var imagesChan = make(chan []byte)

var pc *webrtc.PeerConnection
var vp8Track *webrtc.Track

var screenWidth, screenHeight = screenshot.GetScreenSize()
var resizeWidth, resizeHeight = screenWidth / 2, screenHeight / 2
var encoder, _ = vpxEncoder.NewVpxEncoder(screenWidth, screenHeight, 20, 1200, 5)

var config = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
		// {
		// 	URLs:           []string{"turn:35.247.173.254:3478"},
		// 	Username:       "username",
		// 	Credential:     "password",
		// 	CredentialType: webrtc.ICECredentialTypePassword,
		// },
		// {
		// 	URLs:           []string{"turn:numb.viagenie.ca"},
		// 	Credential:     "muazkh",
		// 	Username:       "webrtc@live.com",
		// 	CredentialType: webrtc.ICECredentialTypePassword,
		// },
	},
}

func Respond(w http.ResponseWriter, data map[string]interface{}) {
	/*This is for response*/
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func Message(status bool, message string) map[string]interface{} {
	return map[string]interface{}{"status": status, "message": message}
}

func connectedClient() *webrtc.SessionDescription {
	peerConnection, err := webrtc.NewPeerConnection(config)
	pc = peerConnection

	if err != nil {
		panic(err)
	}

	// create Trach the we send video back to
	outputTrack, err := peerConnection.NewTrack(webrtc.DefaultPayloadTypeVP8, rand.Uint32(), "video", "pion")
	vp8Track = outputTrack
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
			fmt.Print("Go to read rtp")
			rtp, readErr := track.ReadRTP()
			if readErr != nil {
				panic(readErr)
			}

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

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.SetLocalDescription(offer); err != nil {
		panic(err)
	}
	return &offer
}

func offerHandler(w http.ResponseWriter, r *http.Request) {
	offer := connectedClient()
	// response
	resp := Message(true, "Broadcasting connected !!!")
	resp["offer"] = Session{
		SDP:  offer.SDP,
		Type: offer.Type.String(),
	}
	Respond(w, resp)
	return
}

func answerHandler(w http.ResponseWriter, r *http.Request) {
	session := Session{}
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		Respond(w, Message(false, "Invalid request body"))
		return
	}

	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  session.SDP,
		Type: getType(session.Type),
	}); err != nil {
		fmt.Print(err)
	}

	resp := Message(true, "Answer connected !!!")
	Respond(w, resp)

	go displayWebcam()
	go processImage()

	return
}

func getType(name string) webrtc.SDPType {
	switch name {
	case "offer":
		return webrtc.SDPTypeOffer
	default:
		return webrtc.SDPTypeAnswer
	}
}

func displayWebcam() {
	// get webcam here
	webcam, _ := gocv.OpenVideoCapture(0)
	window := gocv.NewWindow("Hello")
	img := gocv.NewMat()

	defer webcam.Close()
	defer img.Close()

	for {
		webcam.Read(&img)
		window.IMShow(img)
		window.WaitKey(1)

		im, _ := img.ToImage()
		imagesChan <- screenshot.RgbaToYuv(convertImageRGBA(im))
	}
}

func convertImageRGBA(img image.Image) *image.RGBA {
	size := img.Bounds().Size()
	rect := image.Rect(0, 0, size.X, size.Y)
	wImg := image.NewRGBA(rect)

	for y := 0; y < size.Y; y++ {
		// and now loop thorough all of this x's y
		for x := 0; x < size.X; x++ {
			pixel := img.At(x, y)
			//originalColor := img.ColorModel().Convert(pixel).(color.RGBA)
			originalColor := color.RGBAModel.Convert(pixel).(color.RGBA)

			// Offset colors a little, adjust it to your taste
			//r, g, b, a := pixel.RGBA()

			// average
			// grey := uint8((r + g + b) / 3)

			//c := color.RGBA{
			// R: pixel.RGBA()., G: originalColor.G, B: originalColor.B, A: originalColor.A,
			//R: originalColor.R, G: originalColor.G, B: originalColor.B, A: originalColor.A,
			//R: r, G: g, B: b, A: a,
			//}

			wImg.Set(x, y, originalColor)
		}
	}
	return wImg
}

func processImage() {
	go func() {
		for {
			yuv := <-imagesChan
			if len(encoder.Input) < cap(encoder.Input) {
				encoder.Input <- yuv
			}
		}
	}()

	go func() {
		for {
			bs := <-encoder.Output
			vp8Track.WriteSample(media.Sample{Data: bs, Samples: 1})
		}
	}()
}
