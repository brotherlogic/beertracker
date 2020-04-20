package main

import (
	"context"
	"testing"

	"github.com/brotherlogic/keystore/client"

	pb "github.com/brotherlogic/beertracker/proto"
)

func InitTest() *Server {
	s := Init()
	s.GoServer.KSclient = *keystoreclient.GetTestClient("./testing")
	s.GoServer.KSclient.Save(context.Background(), READINGS, &pb.Readings{})
	return s
}

func TestNothing(t *testing.T) {
	doNothing()
}
