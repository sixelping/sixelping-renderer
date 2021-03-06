package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/gorilla/handlers"
	"github.com/sixelping/sixelping-renderer/pkg/mjpeg"
	pb "github.com/sixelping/sixelping-renderer/pkg/sixelping_command"
	"google.golang.org/grpc"
)

var listenFlag = flag.String("listen", ":8081", "Listen address")
var rendererFlag = flag.String("renderer", "localhost:50051", "Renderer address")
var canvasParameters *pb.CanvasParametersResponse

func fetchParameters(client pb.SixelpingRendererClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	parameters, err := client.GetCanvasParameters(ctx, &empty.Empty{})
	if err != nil {
		log.Fatalf("Failed to poll renderer parameters: %v", err)
	}

	canvasParameters = parameters
}

func poller(client pb.SixelpingRendererClient, streamer *mjpeg.Streamer) {
	psd := time.Second / time.Duration(int64(canvasParameters.GetFps()))
	nextTime := time.Now()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		response, err := client.GetRenderedImage(ctx, &empty.Empty{})
		if err == nil {
			bts := response.GetImage()
			streamer.NewFrame(&bts)
		} else {
			log.Fatalf("Failed to poll renderer: %v", err)
		}

		nextTime = nextTime.Add(psd)
		time.Sleep(time.Until(nextTime))
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

	streamer := mjpeg.NewStreamer()
	defer streamer.Close()

	client := pb.NewSixelpingRendererClient(conn)

	fetchParameters(client)
	log.Println("Starting poller...")
	go poller(client, streamer)

	http.Handle("/", handlers.LoggingHandler(os.Stdout, http.HandlerFunc(pageHandler)))
	http.Handle("/stream.mjpeg", handlers.LoggingHandler(os.Stdout, streamer))
	log.Printf("Listening on %s!", *listenFlag)
	log.Fatal(http.ListenAndServe(*listenFlag, nil))
}
