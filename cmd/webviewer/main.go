package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/sixelping/sixelping-renderer/pkg/sixelping_command"
	utils "github.com/sixelping/sixelping-renderer/pkg/sixelping_utils"
	"google.golang.org/grpc"
)

var listenFlag = flag.String("listen", ":8080", "Listen address")
var rendererFlag = flag.String("renderer", "localhost:50051", "Renderer address")
var canvas image.Image
var fps int

func imageHandler(w http.ResponseWriter, r *http.Request) {

	imgBytes := utils.ImageToBytes(canvas)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
	w.Header().Set("Cache-Control", "no-cache, no-store, no-transform")
	w.Write(imgBytes)
}

func poller(client pb.SixelpingRendererClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	parameters, err := client.GetCanvasParameters(ctx, &empty.Empty{})
	if err != nil {
		log.Fatalf("Failed to poll renderer: %v", err)
	}

	canvas = utils.BlackImage(int(parameters.GetWidth()), int(parameters.GetHeight()))
	fps = int(parameters.GetFps())

	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		response, err := client.GetRenderedImage(ctx, &empty.Empty{})
		if err == nil {
			img, _, err := image.Decode(bytes.NewReader(response.GetImage()))
			if err == nil {
				canvas = img
			} else {
				log.Fatalf("Failed to poll renderer: %v", err)
			}
		} else {
			log.Fatalf("Failed to poll renderer: %v", err)
		}

		time.Sleep(time.Second / time.Duration(int64(parameters.GetFps())))
	}
}

func main() {
	flag.Parse()
	log.Println("Connecting to renderer...")
	conn, err := grpc.Dial(*rendererFlag, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("GRPC Fail: %v", err)
	}
	defer conn.Close()

	log.Println("Starting poller...")
	go poller(pb.NewSixelpingRendererClient(conn))

	http.HandleFunc("/", pageHandler)
	http.HandleFunc("/image.png", imageHandler)
	log.Printf("Listening on %s!", *listenFlag)
	log.Fatal(http.ListenAndServe(*listenFlag, nil))
}
