package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/goserver/utils"
	"github.com/brotherlogic/keystore/client"
	"google.golang.org/grpc"

	pb "github.com/brotherlogic/beertracker/proto"
	epb "github.com/brotherlogic/executor/proto"
	pbg "github.com/brotherlogic/goserver/proto"
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

func (s *Server) validate(ctx context.Context) error {
	if _, err := os.Stat("/home/simon/pytilt/pytilt.py"); os.IsNotExist(err) {
		return fmt.Errorf("Cannot locate pytilt binary")
	}

	if _, err := os.Stat("/usr/lib/python2.7/dist-packages/bluetooth/bluez.py"); os.IsNotExist(err) {
		return s.installBluez(ctx)
	}

	if _, err := os.Stat("/usr/lib/python2.7/dist-packages/requests/packages.py"); os.IsNotExist(err) {
		return s.installRequests(ctx)
	}

	return s.checkCap(ctx)
}

func (s *Server) checkCap(ctx context.Context) error {
	conn, err := s.DialServer("executor", s.Registry.Identifier)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	res, err := client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "/sbin/getcap", Parameters: []string{"/usr/bin/python2.7"}}})
	if err != nil {
		return err
	}

	if len(res.GetCommandOutput()) == 0 {
		res, err := client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"/sbin/setcap", "ap_net_raw+eip", "/usr/bin/python2.7"}}})
		if err != nil {
			return err
		}

	}
	return nil
}

func (s *Server) installBluez(ctx context.Context) error {
	conn, err := s.DialServer("executor", s.Registry.Identifier)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	_, err = client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"apt", "install", "-y", "python-bluez"}}})
	return err
}

func (s *Server) installRequests(ctx context.Context) error {
	conn, err := s.DialServer("executor", s.Registry.Identifier)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	_, err = client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "sudo", Parameters: []string{"apt", "install", "-y", "python-requests"}}})
	return err
}

func (s *Server) pullBinaries(ctx context.Context) error {
	conn, err := s.DialServer("executor", s.Registry.Identifier)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := epb.NewExecutorServiceClient(conn)
	_, err = client.Execute(ctx, &epb.ExecuteRequest{Command: &epb.Command{Binary: "git", Parameters: []string{"clone", "https://github.com/brotherlogic/pytilt", "/home/simon/pytilt"}}})
	return err
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
	server.GoServer.KSclient = *keystoreclient.GetClient(server.DialMaster)
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

	server.RegisterRepeatingTaskNonMaster(server.validate, "validate", time.Minute)
	server.RegisterRepeatingTaskNonMaster(server.pullBinaries, "pull_binaries", time.Minute*10)

	fmt.Printf("%v", server.Serve())
}
