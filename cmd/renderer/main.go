package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	empty "github.com/golang/protobuf/ptypes/empty"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	canvaspkg "github.com/sixelping/sixelping-renderer/pkg/canvas"
	pb "github.com/sixelping/sixelping-renderer/pkg/sixelping_command"
	utils "github.com/sixelping/sixelping-renderer/pkg/sixelping_utils"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"google.golang.org/grpc"
)

var canvas *canvaspkg.Canvas
var widthFlag = flag.Int("width", 1920, "Canvas Width")
var heightFlag = flag.Int("height", 1080, "Canvas Height")
var fpsFlag = flag.Int("fps", 1, "Canvas FPS")
var pixTimeoutFlag = flag.Float64("pixeltime", 1.0, "Canvas pixel timeout in seconds")
var listenFlag = flag.String("listen", ":50051", "Listen address")
var promListenFlag = flag.String("mlisten", ":50052", "Metrics listen address")
var logoFlag = flag.String("logo", "", "Logo file")
var promServerFlag = flag.String("prom", "", "Prometheus endpoint to query")
var promDeltasReceived = promauto.NewCounter(prometheus.CounterOpts{
	Name: "renderer_deltas_received_total",
	Help: "Total number of received deltas",
})
var promPacketsReceived *ReceiverMetric
var promPacketsSent *ReceiverMetric
var promPacketsDropped *ReceiverMetric
var promBytesReceived *ReceiverMetric
var promBytesSent *ReceiverMetric
var promPingsReceived *ReceiverPerClientMetric
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

// server is used to implement helloworld.GreeterServer.
type server struct {
	pb.UnimplementedSixelpingRendererServer
}

func (s *server) NewDeltaImage(ctx context.Context, req *pb.NewDeltaImageRequest) (*empty.Empty, error) {
	img := image.NewRGBA(image.Rect(0, 0, *widthFlag, *heightFlag))

	buf := req.GetImage()
	for y := 0; y < *heightFlag; y++ {
		for x := 0; x < *widthFlag; x++ {
			i := y*(*widthFlag) + x
			img.SetRGBA(x, y, color.RGBA{buf[i+2], buf[i+1], buf[i], buf[i+3]})
		}
	}

	err := canvas.AddDelta(img)
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
	for k, v := range req.GetIpcounters() {
		promPingsReceived.Set(req.GetMac(), k, v)
	}
	return &empty.Empty{}, nil
}

func (s *server) GetRenderedImage(ctx context.Context, req *empty.Empty) (*pb.RenderedImageResponse, error) {
	img, err := canvas.GetImage(time.Now())
	if err != nil {
		return nil, err
	}
	return &pb.RenderedImageResponse{Image: utils.ImageToBytes(img)}, nil
}

func queryPrometheus(q string, client api.Client) (float64, error) {
	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := v1api.Query(ctx, q, time.Now())
	if err != nil {
		return 0, err
	}
	if len(warnings) > 0 {
		return 0, errors.New(fmt.Sprintf("Warning: %v", warnings))
	}
	if vec, ok := result.(model.Vector); ok {
		if vec.Len() > 0 {
			return float64(vec[0].Value), nil
		} else {
			return 0, errors.New("No data")
		}
	}
	return 0, errors.New("Invalid result type")
}

func overlayer() {
	var logoImage image.Image
	var client api.Client
	logoImage = nil
	client = nil
	if *logoFlag != "" {
		logoImage = readLogo()
	}

	if *promServerFlag != "" {
		var err error
		client, err = api.NewClient(api.Config{
			Address: *promServerFlag,
		})
		if err != nil {
			log.Fatalf("Error creating client: %v\n", err)
		}
	}

	for {
		overlay := image.NewRGBA(image.Rect(0, 0, *widthFlag, *heightFlag))
		for y := 0; y < *heightFlag; y++ {
			for x := 0; x < *widthFlag; x++ {
				overlay.Set(x, y, image.Transparent)
			}
		}

		if logoImage != nil {
			draw.Draw(overlay, logoImage.Bounds().Add(image.Point{10, *heightFlag - (logoImage.Bounds().Max.Y + 10)}), logoImage, image.ZP, draw.Over)
		}

		if client != nil {
			pps, err := queryPrometheus("sum(receiver_packets_received_per_second)", client)
			if err != nil {
				log.Printf("Prometheus overlay warning: %v", err)
				continue
			}
			dps, err := queryPrometheus("sum(receiver_packets_dropped_per_second)", client)
			if err != nil {
				log.Printf("Prometheus overlay warning: %v", err)
				continue
			}
			bps, err := queryPrometheus("sum(receiver_bits_received_per_second)", client)
			if err != nil {
				log.Printf("Prometheus overlay warning: %v", err)
				continue
			}

			x := 10
			y := *heightFlag - 10

			if logoImage != nil {
				x = x + logoImage.Bounds().Max.X + 10
			}

			text := fmt.Sprintf("%.02f Mpps | %.02f Gbps", pps/1000000.0, bps/1000000000.0)
			col := color.RGBA{255, 255, 255, 255}
			if dps > 0 {
				col = color.RGBA{255, 0, 0, 255}
				text = fmt.Sprintf("%s | Sixelping is currently overloaded and is dropping some of your pings", text)
			}
			point := fixed.Point26_6{fixed.Int26_6(x * 64), fixed.Int26_6(y * 64)}

			d := &font.Drawer{
				Dst:  overlay,
				Src:  image.NewUniform(col),
				Face: basicfont.Face7x13,
				Dot:  point,
			}
			d.DrawString(text)
		}

		canvas.SetOverlayImage(overlay)
		time.Sleep(time.Second)
	}
}

func readLogo() image.Image {
	imageFile, err := os.Open(*logoFlag)
	if err != nil {
		log.Fatalf("Failed to open logo: %v", err)
	}
	defer imageFile.Close()

	img, err := png.Decode(imageFile)
	if err != nil {
		log.Fatalf("Failed to decode logo: %v", err)
	}

	log.Printf("Loaded a %dx%d logo.", img.Bounds().Max.X, img.Bounds().Max.Y)

	return img
}

func setupCanvas() {
	canvas = canvaspkg.NewCanvas(*widthFlag, *heightFlag, uint64((*pixTimeoutFlag)*1000000000))
	go overlayer()
}

func setupMetrics() {
	promPacketsReceived = NewReceiverMetric("receiver_packets_received", "Number of received packets")
	promPacketsSent = NewReceiverMetric("receiver_packets_sent", "Number of sent packets")
	promPacketsDropped = NewReceiverMetric("receiver_packets_dropped", "Number of dropped packets")
	promBytesReceived = NewReceiverMetric("receiver_bytes_received", "Number of received bytes")
	promBytesSent = NewReceiverMetric("receiver_bytes_sent", "Number of sent bytes")
	promPingsReceived = NewReceiverPerClientMetric("receiver_pings_received_per_client", "Pings received per client")

	prometheus.Register(promPacketsReceived)
	prometheus.Register(promPacketsSent)
	prometheus.Register(promPacketsDropped)
	prometheus.Register(promBytesSent)
	prometheus.Register(promBytesReceived)
	prometheus.Register(promPingsReceived)

}

func setupCloseHandler() {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		pprof.StopCPUProfile()
		os.Exit(0)
	}()
}

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		setupCloseHandler()
		defer pprof.StopCPUProfile()
	}
	setupMetrics()
	setupCanvas()
	lis, err := net.Listen("tcp", *listenFlag)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.MaxRecvMsgSize(64000000),
		grpc.MaxSendMsgSize(64000000),
	)
	pb.RegisterSixelpingRendererServer(s, &server{})
	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(s)
	log.Println("Serving requests...")

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(*promListenFlag, nil)
	}()

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
