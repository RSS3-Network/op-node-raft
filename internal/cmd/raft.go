package cmd

import (
	"fmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync"
	"time"
)

const (
	// raftLogCacheSize is the maximum number of logs to cache in-memory.
	// This is used to reduce disk I/O for the recently committed entries.
	raftLogCacheSize = 512

	maxPool = 3
	timeOut = 10 * time.Second
)

var (
	nodeID        string
	advertiseHost string
	advertisePort uint

	joinList []string

	proxy     *httputil.ReverseProxy
	proxyLock sync.RWMutex
)

var raftCmd = &cobra.Command{
	Use: "raft",
	RunE: func(cmd *cobra.Command, args []string) error {
		config := raft.DefaultConfig()
		config.LocalID = raft.ServerID(nodeID)

		raftAddr := fmt.Sprintf("%s:%d", advertiseHost, advertisePort)

		addr, err := net.ResolveTCPAddr("tcp", raftAddr)
		if err != nil {
			return err
		}

		store := raft.NewInmemStore()
		snapshotStore := raft.NewInmemSnapshotStore()
		cacheStore, err := raft.NewLogCache(raftLogCacheSize, store)
		if err != nil {
			return err
		}

		transport, err := raft.NewTCPTransportWithLogger(raftAddr, addr, maxPool, timeOut, hclog.NewNullLogger())
		if err != nil {
			return err
		}

		raftServer, err := raft.NewRaft(config, nil, cacheStore, store, snapshotStore, transport)
		if err != nil {
			return err
		}

		configurations := raft.Configuration{
			Servers: lo.Map(joinList, func(item string, index int) raft.Server {
				return raft.Server{
					ID:      raft.ServerID(strconv.Itoa(index + 1)),
					Address: raft.ServerAddress(fmt.Sprintf("%s:%d", item, advertisePort)),
				}
			}),
		}

		raftServer.BootstrapCluster(configurations)

		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			proxyLock.Lock()
			defer proxyLock.RUnlock()
			if proxy != nil {
				proxy.ServeHTTP(writer, request)
			} else {
				http.Error(writer, "Service Unavailable", http.StatusServiceUnavailable)
			}
		})

		return http.ListenAndServe(":8080", nil)
	},
}

func init() {

	raftCmd.Flags().StringVar(&nodeID, "id", "", "")
	raftCmd.Flags().StringVar(&advertiseHost, "advertise-host", "", "")
	raftCmd.Flags().UintVar(&advertisePort, "advertise-port", 1111, "")

	raftCmd.Flags().StringSliceVar(&joinList, "join", nil, "")
}
