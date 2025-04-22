package raft

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"github.com/raft3d/internal/fsm"
)

type Config struct {
	NodeID            string
	RaftAddr          string
	RaftDir           string
	SnapshotInterval  time.Duration
	SnapshotThreshold uint64
	ClusterNodes      []string
	Bootstrap         bool
}

type Server struct {
	config *Config
	fsm    *fsm.FSM
	raft   *raft.Raft
}

func NewServer(config *Config, fsm *fsm.FSM) (*Server, error) {
	return &Server{
		config: config,
		fsm:    fsm,
	}, nil
}

func parseNodeString(node string) (string, string, error) {
	parts := strings.Split(node, "=")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid node format, expected 'id=addr', got '%s'", node)
	}
	return parts[0], parts[1], nil
}

func (s *Server) Start() error {

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(s.config.NodeID)

	addr, err := net.ResolveTCPAddr("tcp", s.config.RaftAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %v", err)
	}

	transport, err := raft.NewTCPTransport(s.config.RaftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create TCP transport: %v", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(s.config.RaftDir, 3, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %v", err)
	}

	logStorePath := filepath.Join(s.config.RaftDir, "raft-log.db")
	logStore, err := raftboltdb.NewBoltStore(logStorePath)
	if err != nil {
		return fmt.Errorf("failed to create log store: %v", err)
	}

	stableStorePath := filepath.Join(s.config.RaftDir, "raft-stable.db")
	stableStore, err := raftboltdb.NewBoltStore(stableStorePath)
	if err != nil {
		return fmt.Errorf("failed to create stable store: %v", err)
	}

	ra, err := raft.NewRaft(raftConfig, s.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create Raft instance: %v", err)
	}

	s.raft = ra

	if s.config.Bootstrap {
		configuration := raft.Configuration{
			Servers: make([]raft.Server, len(s.config.ClusterNodes)),
		}

		for i, node := range s.config.ClusterNodes {
			id, addr, err := parseNodeString(node)
			if err != nil {
				return fmt.Errorf("failed to parse node string: %v", err)
			}

			configuration.Servers[i] = raft.Server{
				ID:      raft.ServerID(id),
				Address: raft.ServerAddress(addr),
			}
		}

		future := ra.BootstrapCluster(configuration)
		if err := future.Error(); err != nil && err != raft.ErrCantBootstrap {
			return fmt.Errorf("failed to bootstrap cluster: %v", err)
		}
	}

	if s.config.SnapshotInterval > 0 {
		go s.runSnapshotting()
	}

	return nil
}

func (s *Server) runSnapshotting() {
	ticker := time.NewTicker(s.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.raft.State() == raft.Leader {
				future := s.raft.Snapshot()
				if err := future.Error(); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create snapshot: %v\n", err)
				}
			}
		}
	}
}

func (s *Server) Apply(cmd []byte, timeout time.Duration) (interface{}, error) {
	if s.raft.State() != raft.Leader {
		return nil, fmt.Errorf("not the leader")
	}

	future := s.raft.Apply(cmd, timeout)
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to apply command: %v", err)
	}

	resp := future.Response()
	if err, ok := resp.(error); ok {
		return nil, err
	}

	return resp, nil
}

func (s *Server) GetState() raft.RaftState {
	return s.raft.State()
}

func (s *Server) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

func (s *Server) LeaderAddr() string {
	return string(s.raft.Leader())
}

func (s *Server) Shutdown() error {
	future := s.raft.Shutdown()
	return future.Error()
}

func (s *Server) GetNodeID() string {
	return s.config.NodeID
}
