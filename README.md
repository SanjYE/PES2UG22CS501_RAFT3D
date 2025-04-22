# PES2UG22CS501_RAFT3D

# Raft3D - Distributed 3D Printer Management System

Raft3D is a distributed system for managing 3D printers, filaments, and print jobs. It uses the Raft Consensus Algorithm to maintain data consistency across multiple nodes.

## Features

- Fault-tolerant Raft consensus implementation
- Leader election and automatic failover
- RESTful API for managing printers, filaments, and print jobs
- Print job status management with validation rules
- Filament usage tracking

## Running the Application

### Prerequisites

- Go 1.16 or newer
- Network connectivity between nodes

### Building the Application

```bash
go build -o raft3d ./cmd/raft3d
```

### Running a Cluster

To run a 3-node cluster, open three separate terminal windows and run the following commands:

#### Terminal 1 (Node 1 - Bootstrap node)

```bash
./raft3d -id node1 -http 127.0.0.1:8001 -raft 127.0.0.1:7001 -bootstrap -nodes node1=127.0.0.1:7001,node2=127.0.0.1:7002,node3=127.0.0.1:7003
```

#### Terminal 2 (Node 2)

```bash
./raft3d -id node2 -http 127.0.0.1:8002 -raft 127.0.0.1:7002 -nodes node1=127.0.0.1:7001,node2=127.0.0.1:7002,node3=127.0.0.1:7003
```

#### Terminal 3 (Node 3)

```bash
./raft3d -id node3 -http 127.0.0.1:8003 -raft 127.0.0.1:7003 -nodes node1=127.0.0.1:7001,node2=127.0.0.1:7002,node3=127.0.0.1:7003
```

The nodes will automatically elect a leader using the Raft consensus algorithm.

### Testing Leader Election

1. Wait for the nodes to elect a leader (check the console output to see which node is the leader)
2. Use the leader node's HTTP API endpoint to add data (see examples below)
3. Terminate the leader node with Ctrl+C
4. The remaining nodes will elect a new leader
5. Verify that the data is still accessible through the new leader

## API Usage Examples

### Add a new printer

```bash
curl -X POST http://127.0.0.1:8001/api/v1/printers -H "Content-Type: application/json" -d '{
  "id": "printer1",
  "company": "Creality",
  "model": "Ender 3"
}'
```

### List all printers

```bash
curl http://127.0.0.1:8001/api/v1/printers
```

You can also try accessing the API through other nodes:

```bash
curl http://127.0.0.1:8002/api/v1/printers
curl http://127.0.0.1:8003/api/v1/printers
```

### Add a new filament

```bash
curl -X POST http://127.0.0.1:8001/api/v1/filaments -H "Content-Type: application/json" -d '{
  "id": "filament1",
  "type": "PLA",
  "color": "Red",
  "total_weight_in_grams": 1000,
  "remaining_weight_in_grams": 1000
}'
```

### Create a print job

```bash
curl -X POST http://127.0.0.1:8001/api/v1/print_jobs -H "Content-Type: application/json" -d '{
  "id": "job1",
  "printer_id": "printer1",
  "filament_id": "filament1",
  "filepath": "/path/to/model.gcode",
  "print_weight_in_grams": 100
}'
```

### Update print job status

```bash
curl -X POST http://127.0.0.1:8001/api/v1/print_jobs/job1/status -H "Content-Type: application/json" -d '{
  "status": "Running"
}'
```

## Testing Failover

To test failover:

1. Create a printer using the leader node (for example, node1):
   ```bash
   curl -X POST http://127.0.0.1:8001/api/v1/printers -H "Content-Type: application/json" -d '{
     "id": "printer1",
     "company": "Creality",
     "model": "Ender 3"
   }'
   ```

2. Verify the printer is there:
   ```bash
   curl http://127.0.0.1:8001/api/v1/printers
   ```

3. Terminate the leader node (Ctrl+C in the terminal running node1)

4. Wait for a new leader to be elected (check the console output of the remaining nodes)

5. Verify that the printer data is still available using the new leader:
   ```bash
   curl http://127.0.0.1:8002/api/v1/printers
   ```
   or
   ```bash
   curl http://127.0.0.1:8003/api/v1/printers
   ```

This demonstrates that the data is properly replicated and persisted across the cluster using the Raft consensus algorithm.

## Implementation Details

- Raft FSM (Finite State Machine) manages the state of printers, filaments, and print jobs
- Print jobs have a state machine with valid transitions: Queued → Running, Running → Done, Queued → Cancelled, Running → Cancelled
- When a print job is marked as "Done", the system automatically updates the remaining filament weight
- Snapshots are periodically created for fault tolerance
- API endpoints validate inputs and verify entity relationships
