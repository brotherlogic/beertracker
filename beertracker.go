package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beertracker/proto"
	epb "github.com/brotherlogic/executor/proto"
	pbg "github.com/brotherlogic/goserver/proto"
)

var (
	greading = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "beertracker_reading_gravity",
		Help: "Current reading",
	})
	treading = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "beertracker_reading_temperature",
		Help: "Current reading",
	})
	agreading = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "beertracker_auth_gravity",
		Help: "Current reading",
	})
	atreading = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "beertracker_auth_temperature",
		Help: "Current reading",
	})
)

const (
	// READINGS - all beer readings
	READINGS = "/github.com/brotherlogic/beertracker/readings"
)

//Server main server type
type Server struct {
	*goserver.GoServer
}

// Init builds the server
func Init() *Server {
	s := &Server{
		GoServer: &goserver.GoServer{},
	}
	return s
}

// DoRegister does RPC registration
func (s *Server) DoRegister(server *grpc.Server) {

}

// ReportHealth alerts if we're not healthy
func (s *Server) ReportHealth() bool {
	return true
}

// Shutdown the server
func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}

// Mote promotes/demotes this server
func (s *Server) Mote(ctx context.Context, master bool) error {
	return nil
}

// GetState gets the state of the server
func (s *Server) GetState() []*pbg.State {
	return []*pbg.State{
		&pbg.State{Key: "no", Value: int64(233)},
	}
}

func (s *Server) validate() {
	if _, err := os.Stat("/home/simon/pytilt/pytilt.py"); os.IsNotExist(err) {
		s.Log(fmt.Sprintf("Cannot locate pytilt"))
	}

	if _, err := os.Stat("/usr/lib/python2.7/dist-packages/bluetooth/bluez.py"); os.IsNotExist(err) {
		s.installBluez()
	}

	if _, err := os.Stat("/usr/lib/python2.7/dist-packages/requests/packages.py"); os.IsNotExist(err) {
		s.installRequests()
	}

	s.checkCap()
}

//Reading from the tilt
type Reading struct {
	Color     string  `json:"color"`
	Timestamp string  `json:"timestamp"`
	Gravity   int     `json:"gravity"`
	Temp      float32 `json:"temp"`
}

func (s *Server) retrieve() {
	ctx, cancel := utils.ManualContext("bt-ret", "bt-ret", time.Minute, true)
	defer cancel()
	conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
	if err != nil {
		s.Log(fmt.Sprintf("Fatal dial: %v", err))
		return
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	res, err := client.QueueExecute(ctx, &epb.ExecuteRequest{Command: &epb.Command{DeleteOnComplete: true, Binary: "python", Parameters: []string{"/home/simon/pytilt/pytilt.py"}}})
	if err != nil {
		s.Log(fmt.Sprintf("Error on execute: %v", err))
	}

	if len(res.GetCommandOutput()) != 0 {
		var reading Reading
		errj := json.Unmarshal([]byte(strings.Replace(res.GetCommandOutput(), "'", "\"", -1)), &reading)

		//gint, errg := strconv.Atoi(reading.Gravity)
		//tfl, errt := strconv.ParseFloat(reading.Temp, 32)
		s.Log(fmt.Sprintf("Read: %v -> %v (%v, %v) - %v", res.GetCommandOutput(), reading, 1, 1, errj))
		newRead := &pb.Reading{Gravity: int32(reading.Gravity), Timestamp: time.Now().Unix(), Temperature: float32(reading.Temp)}
		greading.Set(float64(reading.Gravity))
		treading.Set(float64(reading.Temp))

		data, _, err := s.KSclient.Read(ctx, READINGS, &pb.Readings{})
		if err != nil {
			s.Log(fmt.Sprintf("Error on read: %v", err))
			return
		}
		readings := data.(*pb.Readings)
		readings.Readings = append(readings.Readings, newRead)

		s.KSclient.Save(ctx, READINGS, readings)
	}
}

func (s *Server) auth(ctx context.Context) (time.Time, error) {
	data, _, err := s.KSclient.Read(ctx, READINGS, &pb.Readings{})
	if err != nil {
		return time.Now().Add(time.Minute), err
	}
	readings := data.(*pb.Readings)
	agreading.Set(float64(readings.GetReadings()[len(readings.GetReadings())-1].Gravity))
	atreading.Set(float64(readings.GetReadings()[len(readings.GetReadings())-1].Temperature))
	return time.Now().Add(time.Minute), nil
}

func (s *Server) checkCap() {
	ctx, cancel := utils.ManualContext("bt-cap", "bt-cap", time.Minute, true)
	defer cancel()
	conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
	if err != nil {
		log.Fatalf("Bad dial of ex: %v", err)
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	res, err := client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "/sbin/getcap", Parameters: []string{"/usr/bin/python2.7"}}})
	if err != nil {
		log.Fatalf("No get cap: %v", err)
	}

	if len(res.GetCommandOutput()) == 0 {
		_, err = client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"/sbin/setcap", "cap_net_raw+eip", "/usr/bin/python2.7"}}})
		if err != nil {
			log.Fatalf("Error in sudo cat: %v", err)
		}

	}
}

func (s *Server) installBluez() {
	ctx, cancel := utils.ManualContext("bt-bluez", "bt-bluez", time.Minute, true)
	defer cancel()
	conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
	if err != nil {
		log.Fatalf("Cannot dial executor: %v", err)
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"apt", "install", "-y", "python-bluez"}}})
}

func (s *Server) installRequests() {
	ctx, cancel := utils.ManualContext("bt-req", "bt-req", time.Minute, true)
	defer cancel()
	conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
	if err != nil {
		log.Fatalf("Cannot dial executor")
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"apt", "install", "-y", "python-requests"}}})
}

func (s *Server) pullBinaries() {
	ctx, cancel := utils.ManualContext("bt-req", "bt-req", time.Minute, true)
	defer cancel()

	conn, err := s.FDialSpecificServer(ctx, "executor", s.Registry.Identifier)
	if err != nil {
		log.Fatalf("Cannot dial server: %v", err)
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	if _, err := os.Stat("/home/simon/pytilt/pytilt.py"); os.IsNotExist(err) {
		client := epb.NewExecutorServiceClient(conn)
		_, err = client.QueueExecute(ctx, &epb.ExecuteRequest{ReadyForDeletion: true, Command: &epb.Command{Binary: "git", Parameters: []string{"clone", "https://github.com/brotherlogic/pytilt", "/home/simon/pytilt"}}})
	} else {
		client.Execute(ctx, &epb.ExecuteRequest{ReadyForDeletion: true, Command: &epb.Command{Binary: "git", Parameters: []string{"--git-dir=/home/simon/pytilt/.git", "pull"}}})
	}
}

func main() {
	var quiet = flag.Bool("quiet", false, "Show all output")
	var init = flag.Bool("init", false, "Prep server")
	flag.Parse()

	//Turn off logging
	if *quiet {
		log.SetFlags(0)
		log.SetOutput(ioutil.Discard)
	}
	server := Init()
	server.PrepServer()
	server.Register = server

	err := server.RegisterServerV2("beertracker", false, true)
	if err != nil {
		return
	}

	if *init {
		ctx, cancel := utils.BuildContext("beertracker", "beertracker")
		defer cancel()

		err := server.KSclient.Save(ctx, READINGS, &pb.Readings{Readings: []*pb.Reading{&pb.Reading{Timestamp: time.Now().Unix()}}})
		fmt.Printf("Initialised: %v\n", err)
		return
	}

	server.pullBinaries()
	server.validate()

	go func() {
		for true {
			time.Sleep(time.Minute)
			server.retrieve()
		}
	}()

	fmt.Printf("%v", server.Serve())
}
