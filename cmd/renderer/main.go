package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"log"
	"net"
	"net/http"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	canvaspkg "github.com/sixelping/sixelping-renderer/pkg/canvas"
	pb "github.com/sixelping/sixelping-renderer/pkg/sixelping_command"
	utils "github.com/sixelping/sixelping-renderer/pkg/sixelping_utils"
	"google.golang.org/grpc"
)

var canvas *canvaspkg.Canvas
var widthFlag = flag.Int("width", 1920, "Canvas Width")
var heightFlag = flag.Int("height", 1080, "Canvas Height")
var fpsFlag = flag.Int("fps", 1, "Canvas FPS")
var pixTimeoutFlag = flag.Float64("pixeltime", 1.0, "Canvas pixel timeout in seconds")
var listenFlag = flag.String("listen", ":50051", "Listen address")
var promListenFlag = flag.String("mlisten", ":50052", "Metrics listen address")
var promDeltasReceived = promauto.NewCounter(prometheus.CounterOpts{
	Name: "renderer_deltas_received_total",
	Help: "Total number of received deltas",
})
var promPacketsReceived *ReceiverMetric
var promPacketsSent *ReceiverMetric
var promPacketsDropped *ReceiverMetric
var promBytesReceived *ReceiverMetric
var promBytesSent *ReceiverMetric

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedSixelpingRendererServer
}

func (s *server) NewDeltaImage(ctx context.Context, req *pb.NewDeltaImageRequest) (*empty.Empty, error) {
	img, _, err := image.Decode(bytes.NewReader(req.GetImage()))
	if err != nil {
		log.Printf("Decoding error: %s", err)
		return nil, err
	}

	err = canvas.AddDelta(img)
	if err != nil {
		return nil, err
	}

	promDeltasReceived.Inc()
	return &empty.Empty{}, nil
}

//Inform others about the canvas parameters
func (s *server) GetCanvasParameters(ctx context.Context, req *empty.Empty) (*pb.CanvasParametersResponse, error) {
	return &pb.CanvasParametersResponse{Width: uint32(*widthFlag), Height: uint32(*heightFlag), Fps: uint32(*fpsFlag)}, nil
}

func (s *server) MetricsUpdate(ctx context.Context, req *pb.MetricsDatapoint) (*empty.Empty, error) {
	promPacketsReceived.Set(req.GetMac(), req.GetIpackets())
	promPacketsSent.Set(req.GetMac(), req.GetOpackets())
	promPacketsDropped.Set(req.GetMac(), req.GetDpackets())
	promBytesReceived.Set(req.GetMac(), req.GetIbytes())
	promBytesSent.Set(req.GetMac(), req.GetObytes())
	return &empty.Empty{}, nil
}

func (s *server) GetRenderedImage(ctx context.Context, req *empty.Empty) (*pb.RenderedImageResponse, error) {
	img, err := canvas.GetImage(time.Now())
	if err != nil {
		return nil, err
	}
	return &pb.RenderedImageResponse{Image: utils.ImageToBytes(img)}, nil
}

func setupCanvas() {
	canvas = canvaspkg.NewCanvas(*widthFlag, *heightFlag, uint64((*pixTimeoutFlag)*1000000000))
}

func setupMetrics() {
	promPacketsReceived = NewReceiverMetric("receiver_packets_received", "Number of received packets")
	promPacketsSent = NewReceiverMetric("receiver_packets_sent", "Number of sent packets")
	promPacketsDropped = NewReceiverMetric("receiver_packets_dropped", "Number of dropped packets")
	promBytesReceived = NewReceiverMetric("receiver_bytes_received", "Number of received bytes")
	promBytesSent = NewReceiverMetric("receiver_bytes_sent", "Number of sent bytes")

	prometheus.Register(promPacketsReceived)
	prometheus.Register(promPacketsSent)
	prometheus.Register(promPacketsDropped)
	prometheus.Register(promBytesSent)
	prometheus.Register(promBytesReceived)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(*promListenFlag, nil)
	}()
}

func main() {
	flag.Parse()
	setupMetrics()
	setupCanvas()
	lis, err := net.Listen("tcp", *listenFlag)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSixelpingRendererServer(s, &server{})
	log.Println("Serving requests...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
