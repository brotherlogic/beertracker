protoc --proto_path ../../../ -I=./proto --go_out=plugins=grpc:./proto proto/beertracker.proto
mv proto/github.com/brotherlogic/beertracker/proto/* ./proto
