package main

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net"
	"net/http"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	pb "github.com/sixelping/sixelping-renderer/pkg/sixelping_command"
	utils "github.com/sixelping/sixelping-renderer/pkg/sixelping_utils"
	"google.golang.org/grpc"
)

var canvas *image.RGBA
var widthFlag = flag.Int("width", 1920, "Canvas Width")
var heightFlag = flag.Int("height", 1080, "Canvas Height")
var fpsFlag = flag.Int("fps", 1, "Canvas FPS")
var listenFlag = flag.String("listen", ":50051", "Listen address")
var promListenFlag = flag.String("mlisten", ":50052", "Metrics listen address")
var dpsFlag = flag.Float64("dps", 0.1, "Darkens per second")
var dinFlag = flag.Int("dint", 16, "Darken intensity")
var binFlag = flag.Int("bint", 8, "Brighten intensity")
var promDeltasReceived = promauto.NewCounter(prometheus.CounterOpts{
	Name: "renderer_deltas_received_total",
	Help: "Total number of received deltas",
})
var dynamicProm = make(map[string]*prometheus.GaugeVec)

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
	mask := image.NewUniform(color.Alpha{uint8(*binFlag)})

	draw.DrawMask(canvas, canvas.Bounds(), img, image.ZP, mask, image.ZP, draw.Over)

	promDeltasReceived.Inc()
	return &empty.Empty{}, nil
}

//Inform others about the canvas parameters
func (s *server) GetCanvasParameters(ctx context.Context, req *empty.Empty) (*pb.CanvasParametersResponse, error) {
	return &pb.CanvasParametersResponse{Width: uint32(*widthFlag), Height: uint32(*heightFlag), Fps: uint32(*fpsFlag)}, nil
}

func (s *server) MetricsUpdate(ctx context.Context, req *pb.MetricsDatapoint) (*empty.Empty, error) {
	for _, e := range req.GetEntry() {
		name := e.GetName()
		lbls := make(prometheus.Labels)
		lblnames := make([]string, len(e.GetLabel()))
		for _, l := range e.GetLabel() {
			lbls[l.GetName()] = l.GetValue()
			lblnames = append(lblnames, l.GetName())
		}

		if _, ok := dynamicProm[name]; !ok {
			dynamicProm[name] = promauto.NewGaugeVec(
				prometheus.GaugeOpts{
					Name: name,
				},
				lblnames)
		}

		dynamicProm[name].With(lbls).Set(float64(e.GetValueU64()))
	}
	return &empty.Empty{}, nil
}

func (s *server) GetRenderedImage(ctx context.Context, req *empty.Empty) (*pb.RenderedImageResponse, error) {
	return &pb.RenderedImageResponse{Image: utils.ImageToBytes(canvas)}, nil
}

func setupCanvas() {
	canvas = utils.BlackImage(*widthFlag, *heightFlag)
	go darkener()
}

func darkener() {
	black := utils.BlackImage(*widthFlag, *heightFlag)
	mask := image.NewUniform(color.Alpha{uint8(*dinFlag)})
	for {
		draw.DrawMask(canvas, canvas.Bounds(), black, image.ZP, mask, image.ZP, draw.Over)
		time.Sleep(time.Duration(float64(time.Second) / (*dpsFlag)))
	}
}

func setupMetrics() {
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
