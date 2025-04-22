package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/raft3d/internal/fsm"
	"github.com/raft3d/pkg/api"
	raft_pkg "github.com/raft3d/pkg/raft"
)

func main() {
	// Parse command line flags
	var (
		nodeID       = flag.String("id", "", "Node ID (must be unique)")
		httpAddr     = flag.String("http", "127.0.0.1:8000", "HTTP API address")
		raftAddr     = flag.String("raft", "127.0.0.1:7000", "Raft address")
		dataDir      = flag.String("data", "data", "Data directory")
		bootstrap    = flag.Bool("bootstrap", false, "Bootstrap the cluster with this node")
		clusterNodes = flag.String("nodes", "", "Comma-separated list of all nodes in the cluster (format: node1=raft_addr1,node2=raft_addr2,...)")
	)
	flag.Parse()

	if *nodeID == "" {
		log.Fatal("Node ID is required")
	}

	// Setup data directory
	nodeDataDir := filepath.Join(*dataDir, *nodeID)
	raftDir := filepath.Join(nodeDataDir, "raft")

	// Create directories if they don't exist
	os.MkdirAll(raftDir, 0755)

	// Parse cluster nodes
	var nodes []string
	if *clusterNodes != "" {
		nodeList := strings.Split(*clusterNodes, ",")
		for _, node := range nodeList {
			parts := strings.Split(node, "=")
			if len(parts) == 1 {
				// If no address is provided, construct the default one
				nodeID := parts[0]
				portBase := 7000
				if nodeID == "node1" {
					portBase = 7001
				} else if nodeID == "node2" {
					portBase = 7002
				} else if nodeID == "node3" {
					portBase = 7003
				}
				nodeAddr := fmt.Sprintf("%s=127.0.0.1:%d", nodeID, portBase)
				nodes = append(nodes, nodeAddr)
			} else {
				nodes = append(nodes, node)
			}
		}
	}

	// Create and initialize Raft FSM and server
	fsmInstance := fsm.NewFSM()

	// Configure Raft server
	raftConfig := &raft_pkg.Config{
		NodeID:            *nodeID,
		RaftAddr:          *raftAddr,
		RaftDir:           raftDir,
		SnapshotInterval:  30 * time.Second,
		SnapshotThreshold: 1000,
		ClusterNodes:      nodes,
		Bootstrap:         *bootstrap,
	}

	// Create Raft server
	raftServer, err := raft_pkg.NewServer(raftConfig, fsmInstance)
	if err != nil {
		log.Fatalf("Failed to create Raft server: %v", err)
	}

	// Start Raft server
	if err := raftServer.Start(); err != nil {
		log.Fatalf("Failed to start Raft server: %v", err)
	}

	// Create and start API server
	apiHandler := api.NewHandler(raftServer, fsmInstance)

	// Capture signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", *httpAddr)
		if err := api.StartServer(apiHandler, *httpAddr); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Print node status information
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				isLeader := raftServer.IsLeader()
				state := raftServer.GetState().String()
				leaderAddr := raftServer.LeaderAddr()

				fmt.Printf("Node %s | State: %s | Leader: %t | Leader Address: %s\n",
					*nodeID, state, isLeader, leaderAddr)
			}
		}
	}()

	// Wait for termination signal
	<-sigCh
	fmt.Println("Shutting down...")

	// Shutdown Raft server
	if err := raftServer.Shutdown(); err != nil {
		log.Printf("Error shutting down Raft server: %v", err)
	}
}
