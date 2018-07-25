package api

import (
	"context"
	"errors"

	log "github.com/Sirupsen/logrus"
	pb "github.com/nre-learning/syringe/api/exp/generated"
	scheduler "github.com/nre-learning/syringe/scheduler"
)

func (s *server) RequestLiveLab(ctx context.Context, lp *pb.LabParams) (*pb.LabUUID, error) {

	// TODO(mierdin): need to perform some basic security checks here. Need to check incoming IP address
	// and do some rate-limiting if possible.

	// Identify lab definition - return error if doesn't exist by ID
	// var labDef *def.LabDefinition
	if _, ok := s.scheduler.LabDefs[lp.LabId]; !ok {
		return &pb.LabUUID{}, errors.New("Failed to find referenced Lab ID")
	}

	// 2 -check to see if it already exists in memory. If it does, don't send provision request.
	// Just look it up and send UUID
	if labUuid, ok := s.sessions[lp.SessionId]; ok {
		return &pb.LabUUID{Id: labUuid}, nil
	}

	// Generate UUID, make sure it doesn't conflict with another (unlikely but easy to check)
	var newUuid string
	for {
		newUuid = GenerateUUID()
		if _, ok := s.liveLabs[newUuid]; !ok {
			break
		}
	}

	// What if one session has multiple UUIDs? Might happen as they transition between labs
	// At the very least you should store a second map inside this map, which contains a map of lab IDs for a given session, and then map those to UUIDs
	//
	// TODO(mierdin): consider not having any tables in memory at all. Just make everything function off of namespace names
	// and literally store all state in kubernetes
	//
	// Ensure sessions table is updated with the new session
	s.sessions[lp.SessionId] = newUuid

	// 3 - if doesn't already exist, put together schedule request and send to channel
	s.scheduler.Requests <- &scheduler.LabScheduleRequest{
		LabDef:    s.scheduler.LabDefs[lp.LabId],
		Operation: scheduler.OperationType_CREATE,
		Uuid:      newUuid,
		Session:   lp.SessionId,
	}

	return &pb.LabUUID{Id: newUuid}, nil
}

func (s *server) DeleteLiveLab(ctx context.Context, lp *pb.LabParams) (*pb.LiveLab, error) {

	// TODO(mierdin): need to perform some security checks here

	if _, ok := s.scheduler.LabDefs[lp.LabId]; !ok {
		return &pb.LiveLab{}, errors.New("Failed to find referenced Lab ID")
	}

	if _, ok := s.sessions[lp.SessionId]; !ok {
		return &pb.LiveLab{}, errors.New("No existing session found to delete")
	}

	// Delete the session
	delete(s.sessions, lp.SessionId)

	s.scheduler.Requests <- &scheduler.LabScheduleRequest{
		LabDef:    s.scheduler.LabDefs[lp.LabId],
		Operation: scheduler.OperationType_DELETE,
		Uuid:      s.sessions[lp.SessionId],
		Session:   lp.SessionId,
	}

	return &pb.LiveLab{}, nil
}

func (s *server) GetLiveLab(ctx context.Context, uuid *pb.LabUUID) (*pb.LiveLab, error) {
	// port1, _ := strconv.Atoi(s.labs[0].LabConnections["csrx1"])

	if uuid.Id == "" {
		msg := "Lab UUID cannot be empty"
		log.Error(msg)
		return nil, errors.New(msg)
	}

	// log.Info(uuid.Id)

	// labMap := map[int]*pb.LiveLab{
	// 	1: &pb.LiveLab{
	// 		LabUUID: 1,
	// 		LabId:   1,
	// 		Endpoints: []*pb.LabEndpoint{
	// 			{
	// 				Name:    "csrx1",
	// 				Type:    pb.LabEndpoint_DEVICE,
	// 				Port:    30005,
	// 				ApiPort: 30005,
	// 			},
	// 			{
	// 				Name:    "csrx2",
	// 				Type:    pb.LabEndpoint_DEVICE,
	// 				Port:    30006,
	// 				ApiPort: 30006,
	// 			},
	// 			{
	// 				Name:    "csrx3",
	// 				Type:    pb.LabEndpoint_DEVICE,
	// 				Port:    30007,
	// 				ApiPort: 30007,
	// 			},
	// 			{
	// 				Name: "j1",
	// 				Type: pb.LabEndpoint_NOTEBOOK,
	// 				Port: 30008,
	// 			},
	// 		},
	// 		Ready: true,
	// 	},
	// }

	log.Infof("About to return %s", s.liveLabs[uuid.Id])
	return s.liveLabs[uuid.Id], nil
}
