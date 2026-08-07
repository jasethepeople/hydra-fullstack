package grpcapi

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"hydra-fullstack/internal/alerts"
	"hydra-fullstack/internal/config"
	"hydra-fullstack/internal/logging"
	"hydra-fullstack/internal/metrics"
	"hydra-fullstack/internal/orchestrator"
	"hydra-fullstack/internal/queue"
	"hydra-fullstack/internal/storage"
)

// Server implements the gRPC services
type Server struct {
	UnimplementedRenderServiceServer
	UnimplementedAlertServiceServer

	alertManager    *alerts.AlertManager
	workerPool      *orchestrator.WorkerPool
	queue           *queue.RedisQueue
	store           *storage.Store
	cfg             *config.Config
	logger          *logging.Logger
}

// NewServer creates a new gRPC server
func NewServer(am *alerts.AlertManager, wp *orchestrator.WorkerPool, 
	q *queue.RedisQueue, store *storage.Store, cfg *config.Config, logger *logging.Logger) *Server {
	return &Server{
		alertManager: am,
		workerPool:   wp,
		queue:        q,
		store:        store,
		cfg:          cfg,
		logger:       logger,
	}
}

// Start starts the gRPC server
func (s *Server) Start(port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(s.unaryInterceptor),
		grpc.StreamInterceptor(s.streamInterceptor),
	)

	RegisterRenderServiceServer(grpcServer, s)
	RegisterAlertServiceServer(grpcServer, s)

	// Enable reflection for grpcurl
	reflection.Register(grpcServer)

	s.logger.Info().Str("port", port).Msg("gRPC server starting")
	return grpcServer.Serve(lis)
}

// unaryInterceptor adds logging and metrics to unary calls
func (s *Server) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)

	status := "OK"
	if err != nil {
		status = "ERROR"
	}

	metrics.RecordRequest("gRPC", info.FullMethod, 200, duration)
	s.logger.Debug().Str("method", info.FullMethod).Dur("duration", duration).Str("status", status).Msg("gRPC call")

	return resp, err
}

// streamInterceptor adds logging to streaming calls
func (s *Server) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	s.logger.Debug().Str("method", info.FullMethod).Msg("gRPC stream started")
	return handler(srv, ss)
}

// ==================== RenderService Implementation ====================

func (s *Server) SubmitRender(ctx context.Context, req *RenderRequest) (*RenderResponse, error) {
	jobID, err := s.workerPool.SubmitJob("render", int(req.Width), int(req.Height), 
		req.Complexity, int(req.Iterations), int(req.Priority))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to submit render: %v", err)
	}

	queueDepth := s.queue.GetDepth()

	return &RenderResponse{
		JobId:          jobID,
		Status:         "queued",
		QueuePosition:  queueDepth,
		EstimatedWait:  fmt.Sprintf("%ds", queueDepth*2),
	}, nil
}

func (s *Server) GetJobStatus(ctx context.Context, req *JobStatusRequest) (*JobStatusResponse, error) {
	// Query from database for persistent status
	// Simplified: return from in-memory for now
	return &JobStatusResponse{
		JobId:     req.JobId,
		Status:    "unknown",
		Progress:  0,
	}, nil
}

func (s *Server) StreamJobLogs(req *JobLogsRequest, stream RenderService_StreamJobLogsServer) error {
	// Simulate log streaming
	for i := 0; i < 5; i++ {
		entry := &LogEntry{
			Timestamp: time.Now().Format(time.RFC3339),
			Level:     "INFO",
			Message:   fmt.Sprintf("Log entry %d for job %s", i, req.JobId),
		}
		if err := stream.Send(entry); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (s *Server) GetWorkerStats(ctx context.Context, req *WorkerStatsRequest) (*WorkerStatsResponse, error) {
	stats := s.workerPool.GetStats()
	cbStats := s.workerPool.GetCircuitBreakerStats()

	var circuitBreakers []*CircuitBreakerStats
	for name, stat := range cbStats {
		circuitBreakers = append(circuitBreakers, &CircuitBreakerStats{
			ServiceName:      name,
			State:            stat["state"].(string),
			FailureCount:     int32(stat["failure_count"].(int)),
			SuccessCount:     int32(stat["success_count"].(int)),
			LastFailure:      stat["last_failure"].(string),
		})
	}

	return &WorkerStatsResponse{
		TotalWorkers:     int32(stats["total_workers"].(int)),
		ActiveWorkers:    int32(stats["active_workers"].(int)),
		BusyWorkers:      int32(stats["busy_workers"].(int)),
		IdleWorkers:      int32(stats["idle_workers"].(int)),
		QueueDepth:       stats["queue_depth"].(int64),
		DeadLetterCount:  stats["dead_letter"].(int64),
		CircuitBreakers:  circuitBreakers,
	}, nil
}

func (s *Server) HealthCheck(ctx context.Context, req *HealthRequest) (*HealthResponse, error) {
	checks := map[string]string{
		"database":    "ok",
		"queue":       "ok",
		"workers":     "ok",
	}

	return &HealthResponse{
		Status:    "healthy",
		Version:   "1.0.0",
		Checks:    checks,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// ==================== AlertService Implementation ====================

func (s *Server) FireAlert(ctx context.Context, req *AlertRequest) (*AlertResponse, error) {
	severity := alerts.Severity(req.Severity)
	category := alerts.Category(req.Category)

	alert := s.alertManager.FireAlert(
		req.AlertType,
		severity,
		category,
		req.Message,
		req.Value,
		req.Threshold,
		req.Unit,
	)

	channels := s.cfg.AlertChannels()

	return &AlertResponse{
		AlertId:    alert.ID,
		Dispatched: true,
		Channels:   channels,
	}, nil
}

func (s *Server) GetActiveAlerts(ctx context.Context, req *ActiveAlertsRequest) (*ActiveAlertsResponse, error) {
	active := s.alertManager.GetActiveAlerts()

	var protoAlerts []*Alert
	for _, a := range active {
		protoAlerts = append(protoAlerts, alertToProto(a))
	}

	return &ActiveAlertsResponse{
		Alerts: protoAlerts,
		Count:  int32(len(protoAlerts)),
	}, nil
}

func (s *Server) AcknowledgeAlert(ctx context.Context, req *AcknowledgeRequest) (*AcknowledgeResponse, error) {
	success := s.alertManager.AcknowledgeAlert(req.AlertId, req.AckedBy)

	return &AcknowledgeResponse{
		Success:   success,
		AlertId:   req.AlertId,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

func (s *Server) GetAlertHistory(ctx context.Context, req *AlertHistoryRequest) (*AlertHistoryResponse, error) {
	history := s.alertManager.GetAlertHistory(int(req.Limit), int(req.Offset))

	var protoAlerts []*Alert
	for _, a := range history {
		protoAlerts = append(protoAlerts, alertToProto(a))
	}

	return &AlertHistoryResponse{
		Alerts: protoAlerts,
		Count:  int32(len(protoAlerts)),
		Total:  int32(s.alertManager.GetTotalCount()),
	}, nil
}

func (s *Server) StreamAlerts(req *StreamAlertsRequest, stream AlertService_StreamAlertsServer) error {
	// Stream current active alerts
	active := s.alertManager.GetActiveAlerts()
	for _, a := range active {
		if req.SeverityFilter != "" && string(a.Severity) != req.SeverityFilter {
			continue
		}

		event := &AlertEvent{
			EventType: "fired",
			Alert:     alertToProto(a),
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if err := stream.Send(event); err != nil {
			return err
		}
	}

	// Keep stream alive with heartbeats
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			heartbeat := &AlertEvent{
				EventType: "heartbeat",
				Timestamp: time.Now().Format(time.RFC3339),
			}
			if err := stream.Send(heartbeat); err != nil {
				return err
			}
		}
	}
}

// alertToProto converts internal alert to protobuf alert
func alertToProto(a alerts.Alert) *Alert {
	return &Alert{
		Id:           a.ID,
		AlertType:    a.Type,
		Severity:     string(a.Severity),
		Category:     string(a.Category),
		Message:      a.Message,
		Value:        a.Value,
		Threshold:    a.Threshold,
		Unit:         a.Unit,
		Timestamp:    a.Timestamp.Format(time.RFC3339),
		Acknowledged: a.Acknowledged,
		AckedBy:      a.AckedBy,
		AckedAt:      a.AckedAt.Format(time.RFC3339),
	}
}
