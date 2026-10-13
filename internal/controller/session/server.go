package session

import (
	"errors"
	"io"
	"log/slog"

	agentv1 "github.com/forlwow/labherd/gen/labherd/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	agentv1.UnimplementedAgentServiceServer
	log *slog.Logger
}

func NewServer(log *slog.Logger) *Server { return &Server{log: log} }

func (s *Server) OpenSession(stream agentv1.AgentService_OpenSessionServer) error {
	hello, err := recvHello(stream)
	if err != nil {
		return err
	}
	log := s.log.With("node_id", hello.GetNodeId(), "instance_id", hello.GetInstanceId())

	err = stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_HelloAck{HelloAck: &agentv1.HelloAck{
			LastAckedSeq: 0,
			ServerTime:   timestamppb.Now(),
		}},
	})
	if err != nil {
		return err
	}

	log.Info(
		"agent connected",
		"hostname", hello.GetHostname(),
		"agent_version", hello.GetAgentVersion(),
		"gpu_count", len(hello.GetGpus()),
	)

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			log.Info("agent closed")
			return nil
		}
		if err != nil {
			log.Info("agent disconnected", "hostname", hello.GetHostname(), "err", err)
			return err
		}
		switch p := req.GetPayload().(type) {
		case *agentv1.OpenSessionRequest_Heartbeat:
			log.Debug("heartbeat", "hostname", hello.GetHostname(), "agent_version", hello.GetAgentVersion())
		case *agentv1.OpenSessionRequest_Metrics:
			if err := handleMetrics(stream, log, p.Metrics); err != nil {
				return err
			}
		case *agentv1.OpenSessionRequest_Processes:
			log.Info("processes", "hostname", hello.GetHostname(), "agent_version", hello.GetAgentVersion(), "count", len(p.Processes.GetProcesses()))
		case *agentv1.OpenSessionRequest_Hello:
			return status.Error(codes.FailedPrecondition, "duplicate hello")
		default:
			log.Warn("unknown payload type")
		}
	}

	return nil
}

func recvHello(stream agentv1.AgentService_OpenSessionServer) (*agentv1.Hello, error) {
	first, err := stream.Recv()
	if err != nil {
		return nil, err
	}
	hello := first.GetHello()
	if hello == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "first message must be hello")
	}
	if hello.GetNodeId() == "" || hello.GetInstanceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "first message must contain node_id and instance_id")
	}
	return hello, nil
}

func handleMetrics(stream agentv1.AgentService_OpenSessionServer, log *slog.Logger, batch *agentv1.MetricsBatch) error {
	for _, sample := range batch.Samples {
		log.Info(
			"metrics received",
			"seq", batch.GetSeq(),
			"collected_at", sample.GetCollectedAt().AsTime(),
			"gpu_count", len(sample.GetGpus()),
		)
	}
	return stream.Send(&agentv1.OpenSessionResponse{
		Payload: &agentv1.OpenSessionResponse_MetricsAck{MetricsAck: &agentv1.MetricsAck{
			AckedSeq: batch.GetSeq(),
		}},
	})
}
